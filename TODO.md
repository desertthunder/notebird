# Notebird Tasks

Build Notebird as a tiny local Go web app/daemon-ish server: a personal wiki with
an old-school Twitter feel.

## Attachments

Use content-addressed storage with SHA-256:

```text
attachments/
  sha256/
    ab/
      cd/
        abcdef...
```

- [ ] Add `attachments` table
- [ ] Add `chirp_attachments` table
- [ ] Store files by SHA-256 hash
- [ ] Add attachment upload endpoint
- [ ] Render image attachments in Markdown/detail views
- [ ] Include attachments in Markdown export

## Import / Export

- [ ] Export Chirps as Markdown files with metadata
- [ ] Import Markdown files into Chirps
- [ ] Preserve tags, fields, timestamps, and refs where possible
- [ ] Decide export layout for attachments

## Observability

- [x] Add structured Charmbracelet logs
- [x] Add request IDs
- [x] Add request timing/status logs
- [x] Add `/healthz`
- [x] Add `/debug/config`
- [x] Add `/debug/vars` via `expvar`
- [ ] Add slow query logging
- [ ] Consider pprof behind an explicit flag

- [ ] Tighten "old-Twitter" visual details
- [ ] Improve mobile collapsed layout
- [ ] Add hover/focus/active polish
- [ ] Add empty/loading/error states for HTMX fragments

## Parking Lot

- Graph view
- Sync
- Plugin system
- True daemon supervision
- File watcher
- OpenTelemetry
- TiddlyWiki import unless cheap
