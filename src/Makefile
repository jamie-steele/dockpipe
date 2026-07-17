# Go build targets — included from repo root Makefile. Run `make` from repository root.
# Paths are relative to repo root (CURDIR when you invoke make from the checkout).

DEB_VERSION ?= $(shell test -f VERSION && tr -d ' \t\r\n' < VERSION || echo 0.6.0)
GO_LDFLAGS := -s -w -X main.Version=$(DEB_VERSION)
WINDOWS_OUT ?= src/bin/dockpipe.exe

.PHONY: build build-core build-windows test-core

# Core DockPipe CLI. Workflows + compiled packages: use ./src/bin/dockpipe --workflow … (same as downstream).
build build-core:
	cp VERSION src/cmd/VERSION
	@mkdir -p src/bin
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o src/bin/dockpipe.bin ./src/cmd
	@cp -f src/bin/dockpipe.bin src/bin/dockpipe
	@chmod +x src/bin/dockpipe
	@echo "Built src/bin/dockpipe.bin — copied to src/bin/dockpipe (use either; bare \`dockpipe\` on PATH may still be an older install)"
	@echo "Install into PATH:  make install   or   make dev-install"

build-windows:
	cp VERSION src/cmd/VERSION
	@mkdir -p "$(dir $(WINDOWS_OUT))"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)" -o "$(WINDOWS_OUT)" ./src/cmd
	@echo "Built $(WINDOWS_OUT) — copy to your Windows machine or PATH"

test-core:
	go test ./...
