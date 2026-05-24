package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	tmplfs "github.com/desertthunder/notebird/internal/templates/html"
)

// controller groups request handlers and rendering helpers for the web UI.
// It is intentionally concrete and package-private: App owns the dependencies,
// while controller keeps HTTP concerns out of the server lifecycle code.
type controller struct {
	*App
}

// newController adapts an App into the handler set used by router.
func newController(app *App) *controller {
	return &controller{App: app}
}

func (ctl *controller) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctl.writeStatus(w, r, http.StatusOK, "ok")
}

func (ctl *controller) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if err := ctl.store.Ping(r.Context()); err != nil {
		ctl.writeStatus(w, r, http.StatusServiceUnavailable, "unavailable")
		return
	}
	ctl.writeStatus(w, r, http.StatusOK, "ready")
}

func (ctl *controller) handleHome(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	filter := feedFilterFromRequest(r)
	chirps, err := ctl.store.ListChirpsFiltered(r.Context(), filter, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.renderChirps(chirps)
	tags, _ := ctl.store.TagCounts(r.Context(), 50)
	wanted, _ := ctl.store.WantedRefs(r.Context())
	data := PageData{
		Chirps:      chirps,
		Selected:    Chirp{},
		CreateForm:  newCreateForm(),
		Tags:        tags,
		WantedRefs:  wanted,
		Filter:      filter,
		Metrics:     pageMetricsSince(start, ctl.recentTemplateMS()),
		CurrentYear: time.Now().Year(),
	}
	ctl.execute(w, "base", data)
}

func (ctl *controller) handleComposerPartial(w http.ResponseWriter, r *http.Request) {
	ctl.executePartial(w, r, "chirp-create", map[string]any{"CreateForm": newCreateForm()})
}

func (ctl *controller) handleNav(w http.ResponseWriter, r *http.Request) {
	tags, err := ctl.store.TagCounts(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	wanted, err := ctl.store.WantedRefs(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.executePartial(w, r, "sidebar", map[string]any{"Tags": tags, "WantedRefs": wanted, "Filter": feedFilterFromRequest(r)})
}

func (ctl *controller) handleFeed(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	filter := feedFilterFromRequest(r)
	chirps, err := ctl.store.ListChirpsFiltered(r.Context(), filter, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.renderChirps(chirps)
	tags, _ := ctl.store.TagCounts(r.Context(), 50)
	wanted, _ := ctl.store.WantedRefs(r.Context())
	ctl.execute(w, "base", PageData{Chirps: chirps, CreateForm: newCreateForm(), Tags: tags, WantedRefs: wanted, Filter: filter, Metrics: pageMetricsSince(start, ctl.recentTemplateMS()), CurrentYear: time.Now().Year()})
}

func (ctl *controller) handleFeedPartial(w http.ResponseWriter, r *http.Request) {
	filter := feedFilterFromRequest(r)
	chirps, err := ctl.store.ListChirpsFiltered(r.Context(), filter, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.renderChirps(chirps)
	w.Header().Set("HX-Push-Url", publicChirpsURL(r))
	ctl.executePartial(w, r, "chirp-list", map[string]any{"Chirps": chirps, "Filter": filter})
}

func (ctl *controller) handleCreateChirp(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	created, err := ctl.store.CreateChirp(r.Context(), r.FormValue("title"), r.FormValue("text"), r.FormValue("tags"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	chirps, err := ctl.store.ListChirps(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.renderChirps(chirps)
	w.Header().Set("HX-Trigger", fmt.Sprintf(`{"notebird:chirp-created":{"id":"%s"},"notebird:nav-refresh":{}}`, created.ID))
	ctl.execute(w, "chirp-list", map[string]any{"Chirps": chirps})
}

func (ctl *controller) handlePreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	refs, err := ctl.store.ResolveTextRefs(r.Context(), r.FormValue("text"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(ctl.renderMarkdown(r.FormValue("text"), refs)))
}

func (ctl *controller) handleSuggest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	kind := r.URL.Query().Get("type")
	q := r.URL.Query().Get("q")
	switch kind {
	case "tag":
		tags, err := ctl.store.SuggestTags(r.Context(), q, 10)
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
		chirps, err := ctl.store.SuggestTitles(r.Context(), q, 10)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		wanted, err := ctl.store.SuggestWantedRefs(r.Context(), q, 10)
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

func (ctl *controller) handleChirpDetail(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	c, err := ctl.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "chirp not found", http.StatusNotFound)
		return
	}
	ctl.renderChirp(&c)
	filter := feedFilterFromRequest(r)
	chirps, err := ctl.store.ListChirpsFiltered(r.Context(), filter, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.renderChirps(chirps)
	tags, _ := ctl.store.TagCounts(r.Context(), 50)
	wanted, _ := ctl.store.WantedRefs(r.Context())
	ctl.execute(w, "base", PageData{Chirps: chirps, Selected: c, CreateForm: newCreateForm(), Tags: tags, WantedRefs: wanted, Filter: filter, Metrics: pageMetricsSince(start, ctl.recentTemplateMS()), CurrentYear: time.Now().Year()})
}

func (ctl *controller) handleChirpDetailPartial(w http.ResponseWriter, r *http.Request) {
	c, err := ctl.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "chirp not found", http.StatusNotFound)
		return
	}
	ctl.renderChirp(&c)
	w.Header().Set("HX-Push-Url", "/chirps/"+c.ID)
	ctl.executePartial(w, r, "chirp-detail", map[string]any{"Selected": c})
}

func (ctl *controller) handleEditChirp(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	c, err := ctl.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "chirp not found", http.StatusNotFound)
		return
	}
	ctl.renderChirp(&c)
	chirps, err := ctl.store.ListChirps(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.renderChirps(chirps)
	tags, _ := ctl.store.TagCounts(r.Context(), 50)
	wanted, _ := ctl.store.WantedRefs(r.Context())
	ctl.execute(w, "base", PageData{Chirps: chirps, Selected: c, CreateForm: newUpdateForm(c), Tags: tags, WantedRefs: wanted, Filter: FeedFilter{Mode: "timeline"}, Metrics: pageMetricsSince(start, ctl.recentTemplateMS()), CurrentYear: time.Now().Year()})
}

func (ctl *controller) handleEditChirpPartial(w http.ResponseWriter, r *http.Request) {
	c, err := ctl.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "chirp not found", http.StatusNotFound)
		return
	}
	w.Header().Set("HX-Push-Url", "/chirps/"+c.ID+"/edit")
	ctl.executePartial(w, r, "chirp-update", map[string]any{"Selected": c, "Form": newUpdateForm(c)})
}

func (ctl *controller) handleUpdateChirp(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	c, err := ctl.store.UpdateChirp(r.Context(), r.PathValue("id"), r.FormValue("title"), r.FormValue("text"), r.FormValue("tags"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctl.renderChirp(&c)
	w.Header().Set("HX-Trigger", `{"notebird:feed-refresh":{},"notebird:nav-refresh":{},"notebird:chirp-saved":{}}`)
	w.Header().Set("HX-Push-Url", "/chirps/"+c.ID)
	ctl.execute(w, "chirp-create", map[string]any{"CreateForm": newCreateForm()})
}

func (ctl *controller) handleDeleteChirp(w http.ResponseWriter, r *http.Request) {
	if err := ctl.store.DeleteChirp(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", `{"notebird:feed-refresh":{},"notebird:nav-refresh":{},"notebird:chirp-deleted":{}}`)
	ctl.execute(w, "chirp-detail", map[string]any{"Selected": Chirp{}})
}

func (ctl *controller) handleUploadAttachment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("attachment")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	if _, err := ctl.store.StoreAttachment(r.Context(), r.PathValue("id"), header.Filename, header.Header.Get("Content-Type"), file); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	chirp, err := ctl.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "chirp not found", http.StatusNotFound)
		return
	}
	ctl.renderChirp(&chirp)
	w.Header().Set("HX-Trigger", `{"notebird:feed-refresh":{}}`)
	ctl.executePartial(w, r, "chirp-detail", map[string]any{"Selected": chirp})
}

func (ctl *controller) handleAttachment(w http.ResponseWriter, r *http.Request) {
	path, contentType, err := ctl.store.AttachmentFile(r.Context(), r.PathValue("hash"))
	if err != nil {
		http.Error(w, "attachment not found", http.StatusNotFound)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "attachment not found", http.StatusNotFound)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, r.URL.Query().Get("name"), time.Now(), file)
}

func (ctl *controller) handleDocs(w http.ResponseWriter, r *http.Request) {
	ctl.execute(w, "docs", nil)
}

func (ctl *controller) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	file, err := tmplfs.Assets.ReadFile("static/openapi.yaml")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(file)
}

func (ctl *controller) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ctl.cfg)
}

func (ctl *controller) writeStatus(w http.ResponseWriter, r *http.Request, status int, state string) {
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": state})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(state + "\n"))
}

func (ctl *controller) execute(w http.ResponseWriter, name string, data any) {
	start := time.Now()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ctl.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Error("template render failed", "template", name, "err", err)
	}
	ctl.lastTemplateMS.Store(uint64(time.Since(start).Microseconds()))
}

func (ctl *controller) executePartial(w http.ResponseWriter, r *http.Request, name string, data any) {
	if !wantsJSON(r) {
		ctl.execute(w, name, data)
		return
	}
	start := time.Now()
	var buf bytes.Buffer
	if err := ctl.templates.ExecuteTemplate(&buf, name, data); err != nil {
		log.Error("template render failed", "template", name, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.lastTemplateMS.Store(uint64(time.Since(start).Microseconds()))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"template": name, "html": buf.String(), "data": data})
}

func (ctl *controller) recentTemplateMS() float64 {
	return float64(ctl.lastTemplateMS.Load()) / 1000
}

func (ctl *controller) renderChirps(chirps []Chirp) {
	for i := range chirps {
		ctl.renderChirpPreview(&chirps[i])
	}
}

func (ctl *controller) renderChirp(c *Chirp) {
	c.HTML = ctl.renderMarkdown(c.Text, c.Refs)
}

func (ctl *controller) renderChirpPreview(c *Chirp) {
	c.HTML = ctl.renderMarkdown(truncateText(c.Text, 320), c.Refs)
}

func (ctl *controller) renderMarkdown(text string, refs []ChirpRef) string {
	var buf bytes.Buffer
	linkedText := RenderWikiLinks(text, refs)
	if err := ctl.markdown.Convert([]byte(linkedText), &buf); err != nil {
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
		TagValue:         strings.Join(c.Tags, ", "),
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

func publicChirpsURL(r *http.Request) string {
	q := r.URL.Query()
	q.Del("json")
	if q.Encode() == "" {
		return "/chirps"
	}
	return "/chirps?" + q.Encode()
}

func wantsJSON(r *http.Request) bool {
	value := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("json")))
	return value == "1" || value == "true" || value == "yes"
}
