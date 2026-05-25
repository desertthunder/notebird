package core

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	tmplfs "github.com/desertthunder/notebird/internal/templates/html"
	"github.com/oklog/ulid/v2"
)

// NoticeType identifies the frontend notification variant to render.
type NoticeType string

const (
	NoticeSuccess NoticeType = "success"
	NoticeError   NoticeType = "error"
	NoticeInfo    NoticeType = "info"
)

// struct Notice is a UI notification delivered through HTMX events.
type Notice struct {
	Type    NoticeType `json:"type"`
	Title   string     `json:"title"`
	Message string     `json:"message"`
}

func notice(ty NoticeType, t, m string) Notice {
	return Notice{Type: ty, Title: t, Message: m}
}

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
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.renderChirps(chirps)
	tags, _ := ctl.store.TagCounts(r.Context(), 50)
	wanted, _ := ctl.store.WantedRefs(r.Context())
	data := PageData{
		Chirps:      chirps,
		Selected:    Chirp{},
		CreateForm:  ctl.newCreateForm(r.Context()),
		Tags:        tags,
		WantedRefs:  wanted,
		Filter:      filter,
		Metrics:     pageMetricsSince(start, ctl.recentTemplateMS()),
		Settings:    ctl.settings(r.Context()),
		CurrentYear: time.Now().Year(),
	}
	ctl.execute(w, "base", data)
}

func (ctl *controller) handleComposerPartial(w http.ResponseWriter, r *http.Request) {
	ctl.executePartial(w, r, "chirp-create", map[string]any{"CreateForm": ctl.newCreateForm(r.Context())})
}

func (ctl *controller) handleNav(w http.ResponseWriter, r *http.Request) {
	tags, err := ctl.store.TagCounts(r.Context(), 50)
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	wanted, err := ctl.store.WantedRefs(r.Context())
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.executePartial(w, r, "sidebar", map[string]any{"Tags": tags, "WantedRefs": wanted, "Filter": feedFilterFromRequest(r)})
}

func (ctl *controller) handleFeed(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	filter := feedFilterFromRequest(r)
	chirps, err := ctl.store.ListChirpsFiltered(r.Context(), filter, 50)
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.renderChirps(chirps)
	tags, _ := ctl.store.TagCounts(r.Context(), 50)
	wanted, _ := ctl.store.WantedRefs(r.Context())
	ctl.execute(w, "base", PageData{Chirps: chirps, CreateForm: ctl.newCreateForm(r.Context()), Tags: tags, WantedRefs: wanted, Filter: filter, Metrics: pageMetricsSince(start, ctl.recentTemplateMS()), Settings: ctl.settings(r.Context()), CurrentYear: time.Now().Year()})
}

func (ctl *controller) handleFeedPartial(w http.ResponseWriter, r *http.Request) {
	filter := feedFilterFromRequest(r)
	chirps, err := ctl.store.ListChirpsFiltered(r.Context(), filter, 50)
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.renderChirps(chirps)
	w.Header().Set("HX-Push-Url", publicChirpsURL(r))
	ctl.executePartial(w, r, "chirp-list", map[string]any{"Chirps": chirps, "Filter": filter})
}

func (ctl *controller) handleCreateChirp(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	created, err := ctl.store.CreateChirp(r.Context(), r.FormValue("title"), r.FormValue("text"), r.FormValue("tags"), r.FormValue("fields"))
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := ctl.store.PromoteDraftAttachments(r.Context(), r.FormValue("draft_id"), created.ID); err != nil {
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	chirps, err := ctl.store.ListChirps(r.Context(), 50)
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.renderChirps(chirps)
	ctl.hx(w, map[string]any{
		"notebird:chirp-created": map[string]string{"id": created.ID},
		"notebird:nav-refresh":   map[string]any{},
		"notebird:notice":        notice(NoticeSuccess, "Chirp posted", "Your note is now in the timeline."),
	})
	ctl.execute(w, "chirp-list", map[string]any{"Chirps": chirps})
}

func (ctl *controller) handlePreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	refs, err := ctl.store.ResolveTextRefs(r.Context(), r.FormValue("text"))
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(ctl.renderMarkdown(r.FormValue("text"), refs)))
}

func (ctl *controller) handleSuggest(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("type")
	q := r.URL.Query().Get("q")
	switch kind {
	case "tag":
		tags, err := ctl.store.SuggestTags(r.Context(), q, 10)
		if err != nil {
			ctl.fail(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items := make([]map[string]string, 0, len(tags))
		for _, tag := range tags {
			items = append(items, map[string]string{"label": "#" + tag, "value": "#" + tag, "detail": "tag"})
		}
		ctl.writeJSON(w, map[string]any{"items": items})
	default:
		chirps, err := ctl.store.SuggestTitles(r.Context(), q, 10)
		if err != nil {
			ctl.fail(w, err.Error(), http.StatusInternalServerError)
			return
		}
		wanted, err := ctl.store.SuggestWantedRefs(r.Context(), q, 10)
		if err != nil {
			ctl.fail(w, err.Error(), http.StatusInternalServerError)
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
		ctl.writeJSON(w, map[string]any{"items": items})
	}
}

func (ctl *controller) handleChirpFields(w http.ResponseWriter, r *http.Request) {
	chirp, err := ctl.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		ctl.fail(w, "chirp not found", http.StatusNotFound)
		return
	}
	ctl.respondFields(w, r, chirp)
}

func (ctl *controller) handleReplaceChirpFields(w http.ResponseWriter, r *http.Request) {
	fields, err := ctl.fieldMapFromRequest(r)
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := ctl.store.ReplaceFields(r.Context(), r.PathValue("id"), fields); err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	chirp, err := ctl.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		ctl.fail(w, "chirp not found", http.StatusNotFound)
		return
	}
	ctl.respondFields(w, r, chirp)
}

func (ctl *controller) handleSetChirpField(w http.ResponseWriter, r *http.Request) {
	value, err := ctl.fieldValueFromRequest(r)
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := ctl.store.SetField(r.Context(), r.PathValue("id"), r.PathValue("key"), value); err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	chirp, err := ctl.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		ctl.fail(w, "chirp not found", http.StatusNotFound)
		return
	}
	ctl.respondFields(w, r, chirp)
}

func (ctl *controller) handleDeleteChirpField(w http.ResponseWriter, r *http.Request) {
	if err := ctl.store.DeleteField(r.Context(), r.PathValue("id"), r.PathValue("key")); err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	chirp, err := ctl.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		ctl.fail(w, "chirp not found", http.StatusNotFound)
		return
	}
	ctl.respondFields(w, r, chirp)
}

func (ctl *controller) respondFields(w http.ResponseWriter, r *http.Request, chirp Chirp) {
	if wantsJSON(r) {
		ctl.writeJSON(w, map[string]any{"fields": chirp.Fields})
		return
	}
	ctl.executePartial(w, r, "chirp-fields", map[string]any{"Selected": chirp})
}

func (ctl *controller) fieldMapFromRequest(r *http.Request) (map[string]string, error) {
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		var payload struct {
			Fields map[string]string `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return nil, err
		}
		if payload.Fields == nil {
			return map[string]string{}, nil
		}
		return payload.Fields, nil
	}
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	return parseFieldInput(r.FormValue("fields"))
}

func (ctl *controller) fieldValueFromRequest(r *http.Request) (string, error) {
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		var payload struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return "", err
		}
		return payload.Value, nil
	}
	if err := r.ParseForm(); err != nil {
		return "", err
	}
	return r.FormValue("value"), nil
}

func (ctl *controller) handleChirpDetail(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	c, err := ctl.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		ctl.fail(w, "chirp not found", http.StatusNotFound)
		return
	}
	ctl.renderChirp(&c)
	filter := feedFilterFromRequest(r)
	chirps, err := ctl.store.ListChirpsFiltered(r.Context(), filter, 50)
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.renderChirps(chirps)
	tags, _ := ctl.store.TagCounts(r.Context(), 50)
	wanted, _ := ctl.store.WantedRefs(r.Context())
	ctl.execute(w, "base", PageData{Chirps: chirps, Selected: c, CreateForm: ctl.newCreateForm(r.Context()), Tags: tags, WantedRefs: wanted, Filter: filter, Metrics: pageMetricsSince(start, ctl.recentTemplateMS()), Settings: ctl.settings(r.Context()), CurrentYear: time.Now().Year()})
}

func (ctl *controller) handleChirpDetailPartial(w http.ResponseWriter, r *http.Request) {
	c, err := ctl.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		ctl.fail(w, "chirp not found", http.StatusNotFound)
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
		ctl.fail(w, "chirp not found", http.StatusNotFound)
		return
	}
	ctl.renderChirp(&c)
	chirps, err := ctl.store.ListChirps(r.Context(), 50)
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.renderChirps(chirps)
	tags, _ := ctl.store.TagCounts(r.Context(), 50)
	wanted, _ := ctl.store.WantedRefs(r.Context())
	ctl.execute(w, "base", PageData{Chirps: chirps, Selected: c, CreateForm: ctl.newUpdateForm(r.Context(), c), Tags: tags, WantedRefs: wanted, Filter: FeedFilter{Mode: "timeline"}, Metrics: pageMetricsSince(start, ctl.recentTemplateMS()), Settings: ctl.settings(r.Context()), CurrentYear: time.Now().Year()})
}

func (ctl *controller) handleEditChirpPartial(w http.ResponseWriter, r *http.Request) {
	c, err := ctl.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		ctl.fail(w, "chirp not found", http.StatusNotFound)
		return
	}
	w.Header().Set("HX-Push-Url", "/chirps/"+c.ID+"/edit")
	ctl.executePartial(w, r, "chirp-update", map[string]any{"Selected": c, "Form": ctl.newUpdateForm(r.Context(), c)})
}

func (ctl *controller) handleUpdateChirp(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	c, err := ctl.store.UpdateChirp(r.Context(), r.PathValue("id"), r.FormValue("title"), r.FormValue("text"), r.FormValue("tags"), r.FormValue("fields"))
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctl.renderChirp(&c)
	ctl.hx(w, map[string]any{
		"notebird:feed-refresh": map[string]any{},
		"notebird:nav-refresh":  map[string]any{},
		"notebird:notice":       notice(NoticeSuccess, "Chirp updated", "Changes were saved."),
	})
	w.Header().Set("HX-Push-Url", "/chirps/"+c.ID)
	ctl.execute(w, "chirp-create", map[string]any{"CreateForm": ctl.newCreateForm(r.Context())})
}

func (ctl *controller) handleDeleteChirp(w http.ResponseWriter, r *http.Request) {
	if err := ctl.store.DeleteChirp(r.Context(), r.PathValue("id")); err != nil {
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.hx(w, map[string]any{
		"notebird:feed-refresh": map[string]any{},
		"notebird:nav-refresh":  map[string]any{},
		"notebird:notice":       notice(NoticeSuccess, "Chirp deleted", "The note was removed."),
	})
	ctl.execute(w, "chirp-detail", map[string]any{"Selected": Chirp{}})
}

func (ctl *controller) handleUploadDraftAttachment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("attachment")
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()
	if _, err := ctl.store.StoreDraftAttachment(r.Context(), r.PathValue("id"), header.Filename, header.Header.Get("Content-Type"), file); err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	attachments, err := ctl.store.ListDraftAttachments(r.Context(), r.PathValue("id"))
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.hx(w, map[string]any{
		"notebird:notice": notice(NoticeSuccess, "Attachment staged", "The file will be linked when you post the Chirp."),
	})
	ctl.executePartial(w, r, "draft-attachments", map[string]any{"DraftID": r.PathValue("id"), "Attachments": attachments})
}

func (ctl *controller) handleUploadAttachment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("attachment")
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	if _, err := ctl.store.StoreAttachment(r.Context(), r.PathValue("id"), header.Filename, header.Header.Get("Content-Type"), file); err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	chirp, err := ctl.store.GetChirp(r.Context(), r.PathValue("id"))
	if err != nil {
		ctl.fail(w, "chirp not found", http.StatusNotFound)
		return
	}
	ctl.renderChirp(&chirp)
	ctl.hx(w, map[string]any{
		"notebird:feed-refresh": map[string]any{},
		"notebird:notice":       notice(NoticeSuccess, "Attachment added", "The file is now linked to this Chirp."),
	})
	ctl.executePartial(w, r, "chirp-detail", map[string]any{"Selected": chirp})
}

func (ctl *controller) handleAttachment(w http.ResponseWriter, r *http.Request) {
	path, contentType, err := ctl.store.AttachmentFile(r.Context(), r.PathValue("hash"))
	if err != nil {
		ctl.fail(w, "attachment not found", http.StatusNotFound)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		ctl.fail(w, "attachment not found", http.StatusNotFound)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, r.URL.Query().Get("name"), time.Now(), file)
}

func (ctl *controller) handleDocs(w http.ResponseWriter, r *http.Request) {
	ctl.execute(w, "docs", nil)
}

func (ctl *controller) handleSettings(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	settings := ctl.settings(r.Context())
	if wantsJSON(r) {
		ctl.writeJSON(w, settings)
		return
	}
	tags, _ := ctl.store.TagCounts(r.Context(), 50)
	wanted, _ := ctl.store.WantedRefs(r.Context())
	ctl.execute(w, "base", PageData{CreateForm: ctl.newCreateForm(r.Context()), Tags: tags, WantedRefs: wanted, Filter: FeedFilter{Mode: "settings"}, Metrics: pageMetricsSince(start, ctl.recentTemplateMS()), Settings: settings, SettingsPage: true, CurrentYear: time.Now().Year()})
}

func (ctl *controller) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		ctl.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	fontSize, _ := strconv.Atoi(r.FormValue("editor_font_size"))
	settings, err := ctl.store.SaveSettings(r.Context(), Settings{EditorMode: r.FormValue("editor_mode"), WordWrap: r.FormValue("word_wrap") == "on" || r.FormValue("word_wrap") == "true", EditorFontSize: fontSize})
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if wantsJSON(r) {
		ctl.writeJSON(w, settings)
		return
	}
	ctl.hx(w, map[string]any{
		"notebird:settings-saved": map[string]any{},
		"notebird:notice":         notice(NoticeSuccess, "Settings saved", "Composer preferences updated."),
	})
	ctl.executePartial(w, r, "settings-panel", map[string]any{"Settings": settings})
}

func (ctl *controller) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	file, err := tmplfs.Assets.ReadFile("static/openapi.yaml")
	if err != nil {
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(file)
}

func (ctl *controller) handleConfig(w http.ResponseWriter, r *http.Request) {
	ctl.writeJSON(w, ctl.cfg)
}

// hx writes a merged HX-Trigger header for backend-driven UI events.
// Handlers use it to ask the browser to refresh fragments and show structured
// notifications without hand-assembling JSON header strings at each call site.
func (ctl *controller) hx(w http.ResponseWriter, events map[string]any) {
	if len(events) == 0 {
		return
	}
	payload, err := json.Marshal(events)
	if err != nil {
		log.Error("encode htmx trigger failed", "err", err)
		return
	}
	w.Header().Set("HX-Trigger", string(payload))
}

// writeJSON writes a JSON response and logs encode failures.
func (ctl *controller) writeJSON(w http.ResponseWriter, value any, status ...int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if len(status) > 0 {
		w.WriteHeader(status[0])
	}
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Error("json response encode failed", "err", err)
	}
}

func (ctl *controller) fail(w http.ResponseWriter, message string, status int) {
	ctl.hx(w, map[string]any{
		"notebird:notice": notice(NoticeError, http.StatusText(status), message),
	})
	http.Error(w, message, status)
}

func (ctl *controller) writeStatus(w http.ResponseWriter, r *http.Request, status int, state string) {
	if wantsJSON(r) {
		ctl.writeJSON(w, map[string]string{"status": state}, status)
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
		ctl.fail(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctl.lastTemplateMS.Store(uint64(time.Since(start).Microseconds()))
	ctl.writeJSON(w, map[string]any{"template": name, "html": buf.String(), "data": data})
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

func (ctl *controller) newCreateForm(ctx context.Context) ChirpForm {
	return ChirpForm{
		Action:           "/chirps",
		Method:           "post",
		SubmitLabel:      "Chirp",
		Heading:          "New Chirp",
		Draft:            true,
		TitlePlaceholder: "Title (optional)",
		TextPlaceholder:  "What's on your mind?",
		DraftID:          ulid.Make().String(),
		Settings:         ctl.settings(ctx),
	}
}

func (ctl *controller) newUpdateForm(ctx context.Context, c Chirp) ChirpForm {
	return ChirpForm{
		Chirp:            c,
		Action:           "/chirps/" + c.ID,
		Method:           "put",
		SubmitLabel:      "Save",
		Heading:          "Editing Chirp",
		Draft:            false,
		ShowCancel:       true,
		TitlePlaceholder: "Title",
		TextPlaceholder:  "Your thoughts..?",
		TagValue:         strings.Join(c.Tags, ", "),
		FieldValue:       formatFieldInput(c.Fields),
		Settings:         ctl.settings(ctx),
	}
}

func (ctl *controller) settings(ctx context.Context) Settings {
	settings, err := ctl.store.GetSettings(ctx)
	if err != nil {
		return defaultSettings
	}
	return settings
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
