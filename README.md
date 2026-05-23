# Notebird

Notebird is a tiny local-first personal wiki that feels like old-school Twitter. It runs as a Go web app on localhost, stores Chirps in SQLite, renders Markdown with wiki links, and uses HTMX/Alpine for small interactive updates instead of a SPA.

## Current shape

- Go server and CLI
- SQLite persistence
- TiddlyWiki-inspired **Chirps** with ULID identity
- Markdown rendering with `[[Wiki Links]]`
- HTMX-powered feed/detail updates
- CodeMirror Markdown composer with server-rendered preview
- Vendored HTMX, Alpine, and local fonts
- Charmbracelet `fang`, `log`, and `lipgloss` for CLI/logging polish

## Requirements

- Go 1.26+
- [`just`](https://github.com/casey/just)
- [`pnpm`](https://pnpm.io/)
- Optional: [`air`](https://github.com/air-verse/air) for Go hot reload

If Air is not installed, `just dev` falls back to `go run github.com/air-verse/air@latest`.

## Run locally

```sh
just run
```

Then open:

```text
http://127.0.0.1:7331
```

By default Notebird stores data in your user config directory under `notebird`. For a throwaway local DB:

```sh
go run ./cmd/notebird --data-dir ./tmp/dev-data
```

## Development

Install and build frontend assets:

```sh
just assets-install
just assets
```

Run with Go hot reload via Air:

```sh
just dev
```

Useful commands:

```sh
just format       # gofmt cmd/internal
just check        # go vet ./...
just test         # go test ./...
just build        # builds ./tmp/notebird, after bundling frontend assets
just assets-watch # watches frontend/src and rebuilds static/dist/app.js
```

## Frontend toolchain

Frontend code lives in `frontend/` and is bundled with pnpm + esbuild into Go's embedded static directory:

```text
frontend/src/composer.js
  -> internal/templates/html/static/dist/app.js
```

The composer uses CodeMirror 6 with Markdown support and common code-language highlighting. Markdown preview is rendered by the Go server through `POST /preview`, so the preview matches saved Chirp rendering, including wiki-link handling.

## Project layout

```text
cmd/notebird/                         CLI entrypoint
internal/cli/                         fang/cobra command setup
internal/core/                        app, HTTP handlers, SQLite store, Chirp model
internal/templates/html/              embedded templates, CSS, vendored JS/fonts, dist assets
frontend/                             CodeMirror/esbuild frontend bundle
```

## Build

```sh
just build
./tmp/notebird
```

The binary starts the local server. `notebird serve` is also supported.
