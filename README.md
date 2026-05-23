# Notebird

Notebird is a tiny local-first personal wiki designed to resemble old-school Twitter.

It runs as a Go web app on localhost, stores Chirps in SQLite, renders Markdown with
wiki links, and uses HTMX/Alpine for small interactive updates instead of a SPA.

## Requirements

- Go 1.26+
- [`just`](https://github.com/casey/just)
- [`pnpm`](https://pnpm.io/)
- Optional: [`air`](https://github.com/air-verse/air) for Go hot reload

If Air is not installed, `just dev` falls back to `go run github.com/air-verse/air@latest`.

### Stack

- Go server and CLI
- SQLite persistence
- TiddlyWiki-inspired **Chirps** with ULID identity
- Markdown rendering with `[[Wiki Links]]`
- HTMX-powered feed/detail updates
- CodeMirror Markdown composer with server-rendered preview
- Vendored HTMX, Alpine, and local fonts
- Charmbracelet `fang`, `log`, and `lipgloss` for CLI/logging polish

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

Run `just` to view all available commands.
