# Notebird Tasks

Build Notebird as a tiny local Go web app/daemon-ish server: a personal wiki with
an old-school Twitter feel.

## Import / Export

- [ ] Export Chirps as Markdown files with metadata
- [ ] Import Markdown files into Chirps
- [ ] Preserve tags, fields, timestamps, and refs where possible
- [ ] Decide export layout for attachments
- [ ] Include attachments in Markdown export

## Observability

- [x] Add structured Charmbracelet logs
- [x] Add request IDs
- [x] Add request timing/status logs
- [x] Add `/healthz`
- [x] Add `/debug/config`
- [x] Add `/debug/vars` via `expvar`
- [ ] Add slow query logging
- [ ] Consider pprof behind an explicit flag

## Parking Lot

- Graph view
- Sync
- Plugin system
- True daemon supervision
- File watcher
- OpenTelemetry
- TiddlyWiki import unless cheap
