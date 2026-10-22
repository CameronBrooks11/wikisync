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

[doc("Push round-trip against a throwaway MediaWiki in Docker (what CI runs)")]
test-integration:
    #!/usr/bin/env bash
    set -euo pipefail
    trap 'scripts/test-wiki.sh down' EXIT
    exports=$(scripts/test-wiki.sh up) # a failed setup stops here
    eval "$exports"
    WIKISYNC_REQUIRE_INTEGRATION=1 go test -tags integration -run TestLivePushRoundTrip -count=1 -v ./internal/core/

[doc("Push round-trip against a real wiki; needs WIKISYNC_TEST_PAGE, WIKISYNC_USERNAME, WIKISYNC_PASSWORD")]
test-live:
    WIKISYNC_REQUIRE_INTEGRATION=1 go test -tags integration -run TestLivePushRoundTrip -count=1 -v ./internal/core/

# Build the binary into bin/
build:
    go build -o bin/wikisync .

# Run the CLI
run *args:
    go run . {{args}}

# Remove build artifacts
clean:
    rm -rf bin/
