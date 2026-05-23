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
	"time"

	"github.com/charmbracelet/log"
	tmplfs "github.com/desertthunder/notebird/internal/templates/html"
	"github.com/desertthunder/notebird/internal/utils"
	"github.com/yuin/goldmark"
)

type App struct {
	cfg       Config
	store     *Store
	templates *template.Template
	markdown  goldmark.Markdown
}

func New(cfg Config) (*App, error) {
	store, err := OpenStore(context.Background(), cfg.DataDir)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("notebird").Funcs(template.FuncMap{
		"safe": func(s string) template.HTML { return template.HTML(s) },
		"shortID": func(s string) string {
			if len(s) <= 8 {
				return s
			}
			return s[:8]
		},
		"timeAgo": utils.TimeAgo,
	}).ParseFS(
		tmplfs.Assets,
		"templates/*.html",
		"templates/layouts/*.html",
		"templates/partials/*.html",
	)
	if err != nil {
		store.Close()
		return nil, err
	}

	return &App{cfg: cfg, store: store, templates: tmpl, markdown: goldmark.New()}, nil
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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return ctx.Err()
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

	mux.HandleFunc("GET /", a.handleHome)
	mux.HandleFunc("POST /chirps", a.handleCreateChirp)
	mux.HandleFunc("GET /chirps/{id}", a.handleChirpDetail)

	return a.requestLogger(mux)
}

func (a *App) handleHome(w http.ResponseWriter, r *http.Request) {
	chirps, err := a.store.ListChirps(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.renderChirps(chirps)
	data := map[string]any{
		"Chirps":   chirps,
		"Selected": firstChirp(chirps),
	}
	a.execute(w, "base", data)
}

// TODO: these could become an internal Controller-like struct or interface.
func (a *App) handleCreateChirp(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := a.store.CreateChirp(r.Context(), r.FormValue("title"), r.FormValue("text")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	chirps, err := a.store.ListChirps(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.renderChirps(chirps)
	w.Header().Set("HX-Trigger", "notebird:chirp-created")
	a.execute(w, "feed", map[string]any{"Chirps": chirps})
}

func (a *App) handleChirpDetail(w http.ResponseWriter, r *http.Request) {
	c, err := a.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "chirp not found", http.StatusNotFound)
		return
	}
	a.renderChirp(&c)
	a.execute(w, "detail", map[string]any{"Selected": c})
}

func (a *App) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(a.cfg)
}

func (a *App) execute(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Error("template render failed", "template", name, "err", err)
	}
}

func (a *App) renderChirps(chirps []Chirp) {
	for i := range chirps {
		a.renderChirp(&chirps[i])
	}
}

func (a *App) renderChirp(c *Chirp) {
	var buf bytes.Buffer
	if err := a.markdown.Convert([]byte(c.Text), &buf); err != nil {
		c.HTML = template.HTMLEscapeString(c.Text)
		return
	}
	c.HTML = buf.String()
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
