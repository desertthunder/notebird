# Notebird Tasks

Build Notebird as a tiny local Go web app/daemon-ish server: a personal wiki with
an old-school Twitter feel.

## Fields

- [x] Make custom Chirp fields first-class in backend APIs
- [x] Render custom fields in Chirp detail views as a compact metadata table
- [x] Show common fields such as `kind`, `status`, `project`, `source`, `rating`, and
  `due` as badges or summary metadata where useful
- [x] Add field editing through frontmatter first
- [x] Add form-based field editing
- [x] Add indexes for field lookups
- [ ] Add inline field editing from Chirp detail views
- [ ] Add field key/value suggestions
- [ ] Add richer field rendering for URLs, dates, and ratings
- [ ] Decide whether any field types need validation beyond string storage

## Query Language and Search

- [ ] Add a Notebird-native filter/query language
- [ ] Support query filters for tags, fields, missing fields, text, sorting, and limits
- [ ] Keep existing `q`, `tag`, and `mode` feed filters working alongside query filters
- [ ] Enhance the existing search input to accept query filters like
  `tag:task status:todo sort:-updated`
- [ ] Add a dedicated search page for composing, explaining, and saving query filters
- [ ] Add backend tests for query parsing and SQL/store results

## Transclusion

- [ ] Add transclusion syntax for Chirps and fields: `{{Title}}`, `{{Title!!field}}`
- [ ] Add recursion/depth protection for transclusion rendering
- [ ] Resolve transclusions in backend Markdown rendering
- [ ] Render missing or circular transclusions as explicit inline errors
- [ ] Render transcluded Chirps with a minimal backend-provided wrapper that can be
  styled later

## Dynamic Lists

- [ ] Add dynamic list blocks backed by query filters
- [ ] Render dynamic lists as Markdown-compatible wiki-link lists initially
- [ ] Add backend tests for dynamic list rendering

## Link Discovery

- [ ] Combine wanted and orphan discovery into one backend-backed link discovery view
- [ ] Add wanted-link grouping by missing title
- [ ] Add orphan Chirps mode for Chirps with no backlinks
- [ ] Add create-missing-Chirp backend flow

## Tags

- [ ] Add tag landing pages backed by store/API support
- [ ] Support tag metadata via regular Chirps with fields like `kind: tag`
- [ ] Support slash-based tag hierarchy such as `work/project-a`

## Story River

- [ ] Define Story River as the ordered set of Chirps the user has opened, pinned, or
  kept in context
- [ ] Add backend model for story items with Chirp ID, position, pinned state, and
  opened timestamp
- [ ] Add store methods to list, open, close, pin, unpin, reorder, and clear story items
- [ ] Add routes/API endpoints for story state before changing the main layout
- [ ] Render story items as backend partials that can replace or augment the current
  single-Chirp detail panel
- [ ] Keep the existing feed/detail workflow working while Story River is introduced

## Import / Export

- [ ] Export Chirps as Markdown files with metadata
- [ ] Import Markdown files into Chirps
- [ ] Preserve tags, fields, timestamps, and refs where possible
- [ ] Preserve transclusion and dynamic-list syntax as Markdown-compatible text
- [ ] Decide export layout for attachments
- [ ] Include attachments in Markdown export
- [ ] Add TiddlyWiki `.tid` import/export
- [ ] Add TiddlyWiki JSON import/export

## Observability

- [x] Add structured Charmbracelet logs
- [x] Add request IDs
- [x] Add request timing/status logs
- [x] Add `/healthz`
- [x] Add `/debug/config`
- [x] Add `/debug/vars` via `expvar`

---

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
