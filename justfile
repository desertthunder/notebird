set shell := ["bash", "-cu"]

binary := "notebird"
out_dir := "tmp"
out := out_dir + "/" + binary

# List available commands
_default:
    just --list

# Install frontend dependencies
assets-install:
    pnpm --dir frontend install

# Bundle frontend assets into the embedded static directory
assets:
    pnpm --dir frontend run build

# Watch and rebuild frontend assets
assets-watch:
    pnpm --dir frontend run dev

# Format Go code
format:
    gofmt -w cmd internal

# Run go vet
check:
    go vet ./...

# Run tests
test:
    go test ./...

# Build the application into ./tmp
build: assets-install assets
    mkdir -p {{out_dir}}
    go build -o {{out}} ./cmd/notebird

# Run the development server without hot reload
run: assets-install assets
    go run ./cmd/notebird

# Run the development server with Air hot reload
dev:
    just assets-install
    if command -v air >/dev/null 2>&1; then air -c .air.toml; else go run github.com/air-verse/air@latest -c .air.toml; fi
