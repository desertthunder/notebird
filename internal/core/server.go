package core

import (
	"bytes"
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	tmplfs "github.com/desertthunder/notebird/internal/templates/html"
	"github.com/desertthunder/notebird/internal/utils"
	"github.com/yuin/goldmark"
	ghtml "github.com/yuin/goldmark/renderer/html"
)

type App struct {
	cfg            Config
	store          *Store
	templates      *template.Template
	markdown       goldmark.Markdown
	lastTemplateMS atomic.Uint64
}

type PageMetrics struct {
	PageMS     float64
	TemplateMS float64
}

type PageData struct {
	Chirps      []Chirp
	Selected    Chirp
	CreateForm  ChirpForm
	Tags        []TagCount
	WantedRefs  []ChirpRef
	Filter      FeedFilter
	Metrics     PageMetrics
	CurrentYear int
}

func New(cfg Config) (*App, error) {
	store, err := OpenStore(context.Background(), cfg.DataDir)
	if err != nil {
		return nil, err
	}

	tmpl := template.New("notebird").Funcs(template.FuncMap{
		"safe":    func(s string) template.HTML { return template.HTML(s) },
		"shortID": shortID,
		"timeAgo": utils.TimeAgo,
	})
	patterns, err := tmplfs.TemplatePatterns(tmplfs.Assets, "templates")
	if err != nil {
		store.Close()
		return nil, err
	}
	tmpl, err = tmpl.ParseFS(tmplfs.Assets, patterns...)
	if err != nil {
		store.Close()
		return nil, err
	}

	return &App{cfg: cfg, store: store, templates: tmpl, markdown: goldmark.New(goldmark.WithRendererOptions(ghtml.WithUnsafe()))}, nil
}

func (a *App) Close() error { return a.store.Close() }

func (a *App) Serve(ctx context.Context) error {
	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", a.cfg.Host, a.cfg.Port),
		Handler:           a.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		log.Info("server listening", "addr", srv.Addr, "data_dir", a.cfg.DataDir)
		errc <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown requested")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("graceful shutdown failed", "err", err)
			return err
		}
		log.Info("server stopped")
		return nil
	case err := <-errc:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	staticFS, _ := fs.Sub(tmplfs.Assets, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok\n")) })
	mux.HandleFunc("GET /debug/config", a.handleConfig)
	mux.Handle("GET /debug/vars", expvar.Handler())
	mux.HandleFunc("GET /docs", a.handleDocs)
	mux.HandleFunc("GET /openapi.yaml", a.handleOpenAPI)

	mux.HandleFunc("GET /", a.handleHome)
	mux.HandleFunc("GET /nav", a.handleNav)
	mux.HandleFunc("GET /chirps", a.handleFeed)
	mux.HandleFunc("POST /chirps", a.handleCreateChirp)
	mux.HandleFunc("POST /preview", a.handlePreview)
	mux.HandleFunc("GET /suggest", a.handleSuggest)
	mux.HandleFunc("GET /chirps/{id}", a.handleChirpDetail)
	mux.HandleFunc("GET /chirps/{id}/edit", a.handleEditChirp)
	mux.HandleFunc("PUT /chirps/{id}", a.handleUpdateChirp)
	mux.HandleFunc("DELETE /chirps/{id}", a.handleDeleteChirp)

	return a.requestLogger(mux)
}

func (a *App) handleHome(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	filter := feedFilterFromRequest(r)
	chirps, err := a.store.ListChirpsFiltered(r.Context(), filter, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.renderChirps(chirps)
	tags, _ := a.store.TagCounts(r.Context(), 50)
	wanted, _ := a.store.WantedRefs(r.Context())
	data := PageData{
		Chirps:      chirps,
		Selected:    firstChirp(chirps),
		CreateForm:  newCreateForm(),
		Tags:        tags,
		WantedRefs:  wanted,
		Filter:      filter,
		Metrics:     PageMetrics{PageMS: float64(time.Since(start).Microseconds()) / 1000, TemplateMS: a.recentTemplateMS()},
		CurrentYear: time.Now().Year(),
	}
	a.execute(w, "base", data)
}

func (a *App) handleNav(w http.ResponseWriter, r *http.Request) {
	tags, err := a.store.TagCounts(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	wanted, err := a.store.WantedRefs(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.execute(w, "sidebar", map[string]any{"Tags": tags, "WantedRefs": wanted, "Filter": feedFilterFromRequest(r)})
}

func (a *App) handleFeed(w http.ResponseWriter, r *http.Request) {
	filter := feedFilterFromRequest(r)
	chirps, err := a.store.ListChirpsFiltered(r.Context(), filter, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.renderChirps(chirps)
	a.execute(w, "chirp-list", map[string]any{"Chirps": chirps, "Filter": filter})
}

// TODO: these could become an internal Controller-like struct or interface.
func (a *App) handleCreateChirp(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	created, err := a.store.CreateChirp(r.Context(), r.FormValue("title"), r.FormValue("text"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	chirps, err := a.store.ListChirps(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.renderChirps(chirps)
	w.Header().Set("HX-Trigger", fmt.Sprintf(`{"notebird:chirp-created":{"id":"%s"},"notebird:nav-refresh":{}}`, created.ID))
	a.execute(w, "chirp-list", map[string]any{"Chirps": chirps})
}

func (a *App) handlePreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	refs, err := a.store.ResolveTextRefs(r.Context(), r.FormValue("text"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(a.renderMarkdown(r.FormValue("text"), refs)))
}

func (a *App) handleSuggest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	kind := r.URL.Query().Get("type")
	q := r.URL.Query().Get("q")
	switch kind {
	case "tag":
		tags, err := a.store.SuggestTags(r.Context(), q, 10)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items := make([]map[string]string, 0, len(tags))
		for _, tag := range tags {
			items = append(items, map[string]string{"label": "#" + tag, "value": "#" + tag, "detail": "tag"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	default:
		chirps, err := a.store.SuggestTitles(r.Context(), q, 10)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		wanted, err := a.store.SuggestWantedRefs(r.Context(), q, 10)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items := make([]map[string]string, 0, len(chirps)+len(wanted))
		seen := map[string]bool{}
		for _, chirp := range chirps {
			seen[chirp.Title] = true
			items = append(items, map[string]string{"label": chirp.Title, "value": chirp.Title, "detail": shortID(chirp.ID)})
		}
		for _, ref := range wanted {
			if seen[ref.RefText] {
				continue
			}
			items = append(items, map[string]string{"label": ref.RefText, "value": ref.RefText, "detail": "wanted link"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	}
}

func (a *App) handleChirpDetail(w http.ResponseWriter, r *http.Request) {
	c, err := a.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "chirp not found", http.StatusNotFound)
		return
	}
	a.renderChirp(&c)
	a.execute(w, "chirp-detail", map[string]any{"Selected": c})
}

func (a *App) handleEditChirp(w http.ResponseWriter, r *http.Request) {
	c, err := a.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "chirp not found", http.StatusNotFound)
		return
	}
	a.execute(w, "chirp-update", map[string]any{"Selected": c, "Form": newUpdateForm(c)})
}

func (a *App) handleUpdateChirp(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	c, err := a.store.UpdateChirp(r.Context(), r.PathValue("id"), r.FormValue("title"), r.FormValue("text"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.renderChirp(&c)
	w.Header().Set("HX-Trigger", `{"notebird:feed-refresh":{},"notebird:nav-refresh":{},"notebird:chirp-saved":{}}`)
	a.execute(w, "chirp-detail", map[string]any{"Selected": c})
}

func (a *App) handleDeleteChirp(w http.ResponseWriter, r *http.Request) {
	if err := a.store.DeleteChirp(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", `{"notebird:feed-refresh":{},"notebird:nav-refresh":{},"notebird:chirp-deleted":{}}`)
	a.execute(w, "chirp-detail", map[string]any{"Selected": Chirp{}})
}

func (a *App) handleDocs(w http.ResponseWriter, r *http.Request) {
	a.execute(w, "docs", nil)
}

func (a *App) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	file, err := tmplfs.Assets.ReadFile("static/openapi.yaml")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(file)
}

func (a *App) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(a.cfg)
}

func (a *App) execute(w http.ResponseWriter, name string, data any) {
	start := time.Now()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Error("template render failed", "template", name, "err", err)
	}
	a.lastTemplateMS.Store(uint64(time.Since(start).Microseconds()))
}

func (a *App) recentTemplateMS() float64 {
	return float64(a.lastTemplateMS.Load()) / 1000
}

func (a *App) renderChirps(chirps []Chirp) {
	for i := range chirps {
		a.renderChirpPreview(&chirps[i])
	}
}

func (a *App) renderChirp(c *Chirp) {
	body, _ := splitFrontmatter(c.Text)
	c.HTML = a.renderMarkdown(body, c.Refs)
}

func (a *App) renderChirpPreview(c *Chirp) {
	body, _ := splitFrontmatter(c.Text)
	c.HTML = a.renderMarkdown(truncateText(body, 320), c.Refs)
}

func (a *App) renderMarkdown(text string, refs []ChirpRef) string {
	var buf bytes.Buffer
	linkedText := RenderWikiLinks(text, refs)
	if err := a.markdown.Convert([]byte(linkedText), &buf); err != nil {
		return template.HTMLEscapeString(text)
	}
	return buf.String()
}

func truncateText(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "…"
}

func newCreateForm() ChirpForm {
	return ChirpForm{
		Action:           "/chirps",
		Method:           "post",
		SubmitLabel:      "Chirp",
		Heading:          "New Chirp",
		Draft:            true,
		TitlePlaceholder: "Title, optional",
		TextPlaceholder:  "What are you noticing? Markdown, fenced code, wiki links, and #tags work.",
	}
}

func newUpdateForm(c Chirp) ChirpForm {
	return ChirpForm{
		Chirp:            c,
		Action:           "/chirps/" + c.ID,
		Method:           "put",
		SubmitLabel:      "Save",
		Heading:          "Editing Chirp",
		Draft:            false,
		ShowCancel:       true,
		TitlePlaceholder: "Title",
		TextPlaceholder:  "Markdown",
	}
}

func feedFilterFromRequest(r *http.Request) FeedFilter {
	q := r.URL.Query()
	filter := FeedFilter{
		Query: q.Get("q"),
		Tag:   q.Get("tag"),
		Mode:  q.Get("mode"),
	}
	if filter.Mode == "" {
		filter.Mode = "timeline"
	}
	return filter
}

func firstChirp(chirps []Chirp) Chirp {
	if len(chirps) == 0 {
		return Chirp{}
	}
	return chirps[0]
}

func (a *App) requestLogger(next http.Handler) http.Handler {
	return loggingMiddleware{}.wrap(next)
}
