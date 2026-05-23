set shell := ["bash", "-cu"]

binary := "notebird"
out_dir := "tmp"
out := out_dir + "/" + binary

# List available commands
_default:
    just --list

format:
    gofmt -w cmd internal

# Run go vet
check:
    go vet ./...

test:
    go test ./...

# Build the application into ./tmp
build:
    mkdir -p {{out_dir}}
    go build -o {{out}} ./cmd/notebird

# Run the development server
run:
    go run ./cmd/notebird
