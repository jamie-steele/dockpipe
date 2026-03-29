# Go build targets — included from repo root Makefile. Run `make` from repository root.
# Paths are relative to repo root (CURDIR when you invoke make from the checkout).

DEB_VERSION ?= $(shell test -f VERSION && tr -d ' \t\r\n' < VERSION || echo 0.6.0)
GO_LDFLAGS := -s -w -X main.Version=$(DEB_VERSION)
WINDOWS_OUT ?= src/bin/dockpipe.exe

.PHONY: build build-windows test maintainer-tools

# Core DockPipe CLI. Workflows + compiled packages: use ./src/bin/dockpipe --workflow … (same as downstream).
build:
	cp VERSION src/cmd/VERSION
	@mkdir -p src/bin
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o src/bin/dockpipe.bin ./src/cmd
	@echo "Built src/bin/dockpipe.bin — run via src/bin/dockpipe or make install / make dev-install"
	@echo "Optional: make maintainer-tools (dorkpipe + mcpd under packages/)"

# Maintainer: orchestrator + MCP (separate modules under packages/). Not required for dockpipe core.
maintainer-tools:
	@mkdir -p packages/dorkpipe/bin packages/dockpipe-mcp/bin
	go build -C packages/dorkpipe/lib -trimpath -ldflags "$(GO_LDFLAGS)" -o ../bin/dorkpipe ./cmd/dorkpipe
	go build -C packages/dockpipe-mcp -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/mcpd ./cmd/mcpd
	@echo "Built packages/dorkpipe/bin/dorkpipe and packages/dockpipe-mcp/bin/mcpd"

build-windows:
	cp VERSION src/cmd/VERSION
	@mkdir -p "$(dir $(WINDOWS_OUT))"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)" -o "$(WINDOWS_OUT)" ./src/cmd
	@echo "Built $(WINDOWS_OUT) — copy to your Windows machine or PATH"

test:
	go test ./...
	go test ./packages/dorkpipe/lib/...
	go test ./packages/dockpipe-mcp/...
