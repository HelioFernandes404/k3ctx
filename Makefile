BINARY     := k3ctx
PREFIX     ?= $(HOME)/.local
BIN_DIR    := $(PREFIX)/bin
GO_VERSION := 1.25.0
GO         ?= sh -c 'if command -v mise >/dev/null 2>&1; then exec mise exec go@$(GO_VERSION) -- go "$$@"; else exec go "$$@"; fi' --

.PHONY: test build install clean

test:
	$(GO) test ./...

build:
	mkdir -p bin
	$(GO) build -o bin/$(BINARY) ./cmd/$(BINARY)/

install: build
	mkdir -p $(BIN_DIR)
	install -m 0755 bin/$(BINARY) $(BIN_DIR)/$(BINARY)

clean:
	rm -rf bin/
