# Build the Go CLI into bin/dockpipe.bin (bin/dockpipe launcher invokes it).
# Keep in sync with repo-root VERSION (used by CI / release).
DEB_VERSION ?= $(shell test -f VERSION && tr -d ' \t\r\n' < VERSION || echo 0.6.0)
GO_LDFLAGS := -s -w -X main.Version=$(DEB_VERSION)
.PHONY: build build-windows test deb deb-all
build:
	cp VERSION cmd/dockpipe/VERSION
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/dockpipe.bin ./cmd/dockpipe
	@echo "Built bin/dockpipe.bin — run via bin/dockpipe"

# Cross-compile for Windows (output bin/dockpipe.exe — gitignored). From repo root only.
build-windows:
	cp VERSION cmd/dockpipe/VERSION
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/dockpipe.exe ./cmd/dockpipe
	@echo "Built bin/dockpipe.exe — copy to your Windows machine or PATH"

test:
	go test ./...

# Debian packages (requires dpkg-deb). Default arch amd64; deb-all builds amd64 + arm64.
deb:
	./packaging/build-deb.sh "$(DEB_VERSION)" amd64

deb-all:
	./packaging/build-deb-all.sh "$(DEB_VERSION)"
