# Notebird Plan

Build Notebird as a tiny local Go web app/daemon-ish server: a personal wiki with an
old-school Twitter feel.

## Core Model

Call TiddlyWiki-like tiddlers **Chirps**.

```go
type Chirp struct {
    ID        string            // ULID, canonical identity
    Title     string            // human-facing authoring identity
    Text      string            // Markdown
    Type      string            // text/markdown
    Tags      []string
    Refs      []string          // resolved links to other Chirps
    Fields    map[string]string // EAV-backed extension fields
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

- Use ULIDs internally.
- Let people write human links with `[[Title]]`.
- Store resolved refs by ULID where possible.
- Track unresolved refs as wanted/missing links.
- Render Markdown with `goldmark`.

## Storage

Use SQLite as primary storage, with Markdown import/export later.

Schema direction:

- `chirps`: relational core fields
- `chirp_tags`: normalized tags
- `chirp_refs`: normalized refs/backlinks/wanted links
- `chirp_fields`: cautious EAV for custom fields only
- `attachments` / `chirp_attachments`: content-addressed files
- SQLite FTS5 for search

Avoid pure EAV for the whole app.

## CLI / Daemon Shape

- `notebird` should run the server by default.
- `notebird serve` should run the same server explicitly.
- True background daemon commands are deferred.
- Use Charmbracelet tools:
  - `fang` for CLI execution/styling
  - `log` for structured logs
  - `lipgloss` for terminal styling where useful

## Web Stack

- stdlib `net/http`
- `html/template`
- vendored HTMX
- vendored Alpine
- vanilla CSS
- avoid full page reloads for normal interactions

## UI

Desktop split-pane layout:

- left: nav/search/tags
- center: feed + composer
- right: selected Chirp detail, refs, backlinks

Small screens collapse toward feed-first.

## Design

Capture the aesthetic of old-school Twitter: compact, blue, bordered panes, simple gradients, dense metadata, and a friendly wordmark.

### Fonts

| Role                                | Font                                       | Use                                                               |
| ----------------------------------- | ------------------------------------------ | ----------------------------------------------------------------- |
| **UI text**                         | **Arimo**                                  | Tweets, nav, forms, buttons, metadata, menus                      |
| **Wordmark**                        | **Fredoka**                                | App logo only                                                     |
| **Numbers / counters**              | **Roboto Mono**                            | Character count, stats, timestamps when you want a “utility” feel |
| **Longer notes / article previews** | **Source Sans 3**                          | Expanded detail pane, linked article snippets, help text          |
| **Marketing / splash heading**      | **Nunito Sans**                            | Landing page headings, onboarding cards, empty states             |

### CSS Structure

```text
internal/templates/html/static/styles/
  style.css
  reset.css
  base.css
  utilities.css
  tokens/
    colors.css
    fonts.css
    spacing.css
  components/
    app-shell.css
    composer.css
    chirp.css
    detail.css
```

## v1 Feature Set

- create/edit/delete Chirps
- timeline feed
- split-pane UI
- tags
- wiki links
- backlinks/refs
- missing/wanted links
- Markdown rendering
- SQLite FTS5 search
- favorites/pins via tags/fields or later collection records
- keyboard shortcuts
- autosave drafts
- Markdown import/export
- attachments/images
- local CLI/server/logging

## Attachments

Use content-addressed storage with SHA-256:

```text
attachments/
  sha256/
    ab/
      cd/
        abcdef...
```

## Observability

- structured Charmbracelet logs
- request IDs
- request timing/status logs
- `/healthz`
- `/debug/config`
- `/debug/vars` via `expvar`
- pprof maybe later behind a flag

## Parking Lot

- graph view
- sync
- plugin system
- true daemon supervision
- file watcher
- OpenTelemetry
- TiddlyWiki import unless cheap
