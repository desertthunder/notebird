package core

import (
	"context"
	"expvar"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	tmplfs "github.com/desertthunder/notebird/internal/templates/html"
	"github.com/desertthunder/notebird/internal/utils"
	"github.com/yuin/goldmark"
	ghtml "github.com/yuin/goldmark/renderer/html"
)

// struct App owns application related state, dependencies, and server lifecycle.
// Request handling is delegated to an internal controller created by router.
type App struct {
	cfg            Config
	store          *Store
	templates      *template.Template
	markdown       goldmark.Markdown
	lastTemplateMS atomic.Uint64
}

// PageData is the view model passed to full-page HTML templates.
type PageData struct {
	Chirps     []Chirp
	Selected   Chirp
	CreateForm ChirpForm
	Tags       []TagCount
	WantedRefs []ChirpRef
	Filter     FeedFilter
	// Metrics is expected on every full-page base template render so the footer
	// can report page timing consistently across app pages.
	Metrics      PageMetrics
	Settings     Settings
	SettingsPage bool
	CurrentYear  int
}

// New opens application storage, parses embedded templates, and prepares markdown rendering.
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

// Close releases resources owned by the application.
func (a *App) Close() error { return a.store.Close() }

// Serve starts the HTTP server and shuts it down when ctx is canceled.
func (a *App) Serve(ctx context.Context) error {
	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", a.cfg.Host, a.cfg.Port),
		Handler:           a.router(),
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

// router wires HTTP routes to the internal controller and returns the root handler.
//
// `/x/` routes serve HTML fragments/partials for client-side navigation,
// while other routes serve full pages or APIs.
func (a *App) router() http.Handler {
	mux := http.NewServeMux()
	c := newController(a)

	staticFS, _ := fs.Sub(tmplfs.Assets, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("GET /healthz", c.handleHealth)
	mux.HandleFunc("GET /readyz", c.handleReadiness)
	mux.HandleFunc("GET /debug/config", c.handleConfig)
	mux.Handle("GET /debug/vars", expvar.Handler())
	mux.HandleFunc("GET /docs", c.handleDocs)
	mux.HandleFunc("GET /settings", c.handleSettings)
	mux.HandleFunc("POST /settings", c.handleUpdateSettings)
	mux.HandleFunc("GET /openapi.yaml", c.handleOpenAPI)
	mux.HandleFunc("GET /attachments/{hash}", c.handleAttachment)

	mux.HandleFunc("GET /", c.handleHome)

	mux.HandleFunc("GET /x/nav", c.handleNav)
	mux.HandleFunc("GET /x/composer", c.handleComposerPartial)
	mux.HandleFunc("GET /x/chirps", c.handleFeedPartial)
	mux.HandleFunc("GET /x/chirps/{id}", c.handleChirpDetailPartial)
	mux.HandleFunc("GET /x/chirps/{id}/edit", c.handleEditChirpPartial)

	mux.HandleFunc("GET /chirps", c.handleFeed)
	mux.HandleFunc("POST /chirps", c.handleCreateChirp)
	mux.HandleFunc("POST /preview", c.handlePreview)
	mux.HandleFunc("GET /suggest", c.handleSuggest)
	mux.HandleFunc("GET /chirps/{id}", c.handleChirpDetail)
	mux.HandleFunc("GET /chirps/{id}/edit", c.handleEditChirp)
	mux.HandleFunc("PUT /chirps/{id}", c.handleUpdateChirp)
	mux.HandleFunc("DELETE /chirps/{id}", c.handleDeleteChirp)
	mux.HandleFunc("POST /chirps/{id}/attachments", c.handleUploadAttachment)
	mux.HandleFunc("POST /drafts/{id}/attachments", c.handleUploadDraftAttachment)

	return a.requestLogger(mux)
}

func (a *App) requestLogger(next http.Handler) http.Handler {
	return loggingMiddleware{}.wrap(next)
}
