BINARY     := k3ctx
PREFIX     ?= $(HOME)/.local
BIN_DIR    := $(PREFIX)/bin
GO_VERSION := 1.25.0
GO         ?= sh -c 'if command -v mise >/dev/null 2>&1; then exec mise exec go@$(GO_VERSION) -- go "$$@"; else exec go "$$@"; fi' --
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE       ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -ldflags "-X github.com/systemframe/k3ctx/internal/version.Version=$(VERSION) -X github.com/systemframe/k3ctx/internal/version.Commit=$(COMMIT) -X github.com/systemframe/k3ctx/internal/version.Date=$(DATE)"

.PHONY: test build install clean lint

test:
	$(GO) test ./...

build:
	mkdir -p bin
	$(GO) build $(LDFLAGS) -o bin/$(BINARY) ./cmd/$(BINARY)/

install: build
	mkdir -p $(BIN_DIR)
	install -m 0755 bin/$(BINARY) $(BIN_DIR)/$(BINARY)

clean:
	rm -rf bin/

lint:
	golangci-lint run ./...
