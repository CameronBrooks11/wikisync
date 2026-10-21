# Go task runner

set dotenv-load := false

# Default: show available recipes
default:
    @just --list

# Install dependencies and dev tools
setup:
    go mod download
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1

# Format code (mutates working tree — use locally)
fmt:
    gofmt -w .

# Verify formatting (non-mutating — use in CI)
fmt-check:
    @test -z "$(gofmt -l .)" || { echo "These files need 'just fmt':"; gofmt -l .; exit 1; }

# Run linters
lint:
    golangci-lint run

# Format-check + lint + vet (non-mutating — safe for CI)
check: fmt-check lint
    go vet ./...

# Run tests (race detector on)
test:
    go test -race ./...

# Build the binary into bin/
build:
    go build -o bin/wikisync .

# Run the CLI
run *args:
    go run . {{args}}

# Remove build artifacts
clean:
    rm -rf bin/
