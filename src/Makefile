# Go build targets — included from repo root Makefile. Run `make` from repository root.
# Paths are relative to repo root (CURDIR when you invoke make from the checkout).

DEB_VERSION ?= $(shell test -f VERSION && tr -d ' \t\r\n' < VERSION || echo 0.6.0)
GO_LDFLAGS := -s -w -X main.Version=$(DEB_VERSION)
WINDOWS_OUT ?= src/bin/dockpipe.exe

.PHONY: build build-windows test

build:
	cp VERSION src/cmd/dockpipe/VERSION
	@mkdir -p src/bin
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o src/bin/dockpipe.bin ./src/cmd/dockpipe
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o src/bin/dorkpipe ./src/cmd/dorkpipe
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o src/bin/mcpd ./src/cmd/mcpd
	@echo "Built src/bin/dockpipe.bin — run via src/bin/dockpipe, ./bin/dockpipe (PATH hint), or make install / make dev-install"
	@echo "Built src/bin/dorkpipe (DorkPipe DAG orchestrator on src/lib/dorkpipe)"
	@echo "Built src/bin/mcpd (MCP stdio or HTTPS — see src/lib/mcpbridge/README.md)"

build-windows:
	cp VERSION src/cmd/dockpipe/VERSION
	@mkdir -p "$(dir $(WINDOWS_OUT))"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)" -o "$(WINDOWS_OUT)" ./src/cmd/dockpipe
	@echo "Built $(WINDOWS_OUT) — copy to your Windows machine or PATH"

test:
	go test ./...
