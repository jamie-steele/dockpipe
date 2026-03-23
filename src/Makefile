# Go build targets — included from repo root Makefile. Run `make` from repository root.
# Paths are relative to repo root (CURDIR when you invoke make from the checkout).

DEB_VERSION ?= $(shell test -f VERSION && tr -d ' \t\r\n' < VERSION || echo 0.6.0)
GO_LDFLAGS := -s -w -X main.Version=$(DEB_VERSION)
WINDOWS_OUT ?= bin/dockpipe.exe

.PHONY: build build-windows test

build:
	cp VERSION src/cmd/dockpipe/VERSION
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/dockpipe.bin ./src/cmd/dockpipe
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/dorkpipe ./src/cmd/dorkpipe
	@echo "Built bin/dockpipe.bin — run via bin/dockpipe (repo) or make install / make dev-install"
	@echo "Built bin/dorkpipe (DorkPipe DAG orchestrator on src/lib/dorkpipe)"

build-windows:
	cp VERSION src/cmd/dockpipe/VERSION
	@mkdir -p "$(dir $(WINDOWS_OUT))"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)" -o "$(WINDOWS_OUT)" ./src/cmd/dockpipe
	@echo "Built $(WINDOWS_OUT) — copy to your Windows machine or PATH"

test:
	go test ./...
