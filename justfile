# vt-backup justfile. `just` (https://github.com/casey/just) drives builds.
#
# Pure-Go binary: no CGO, no unixODBC, no container needed. The runtime
# dependency is OpenLink's `isql` binary, which ships with Virtuoso.

set shell := ["bash", "-uc"]

# Version stamped into the binary (overridable: `just version=1.2.3 build`).
version := `git describe --tags --always --dirty 2>/dev/null || echo dev`

ldflags := "-X github.com/kairoaraujo/vt-backup/cmd.Version=" + version

# Default: native build into bin/.
default: build

# Native build for the current host.
build:
    mkdir -p bin
    CGO_ENABLED=0 go build -ldflags "{{ldflags}}" -o bin/vt-backup .

build-linux-amd64:
    mkdir -p bin
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "{{ldflags}}" -o bin/vt-backup-linux-amd64 .

build-linux-arm64:
    mkdir -p bin
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "{{ldflags}}" -o bin/vt-backup-linux-arm64 .

build-darwin-amd64:
    mkdir -p bin
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "{{ldflags}}" -o bin/vt-backup-darwin-amd64 .

build-darwin-arm64:
    mkdir -p bin
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "{{ldflags}}" -o bin/vt-backup-darwin-arm64 .

# Build every supported target. Single host, no Docker required.
build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64

test:
    go test -race ./...

# Race + coverage profile (used by CI; writes coverage.out).
cover:
    go test -race -covermode=atomic -coverprofile=coverage.out ./...

vet:
    go vet ./...

# Static analysis. Requires golangci-lint v2 (https://golangci-lint.run).
lint:
    golangci-lint run

# Full local check bundle: vet, lint, race tests.
check: vet lint test

fmt:
    gofmt -w -s .
    go mod tidy

# Show what's in bin/ with sizes.
bins:
    @ls -lh bin/ 2>/dev/null || echo "no binaries built yet"

# Install the native binary to /usr/local/bin (sudo).
install: build
    sudo install -m 0755 bin/vt-backup /usr/local/bin/vt-backup
    @echo "installed: /usr/local/bin/vt-backup"

# Wipe build artifacts.
clean:
    rm -rf bin/
