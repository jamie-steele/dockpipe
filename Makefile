# Build the Go CLI into bin/dockpipe.bin (bin/dockpipe launcher invokes it).
# Keep in sync with repo-root VERSION (used by CI / release).
DEB_VERSION ?= $(shell test -f VERSION && tr -d ' \t\r\n' < VERSION || echo 0.6.0)
GO_LDFLAGS := -s -w -X main.Version=$(DEB_VERSION)
# Windows exe output path. smb://… is not a filesystem path—mount the share in Finder / Files first, then use the mount path, e.g.
#   make build-windows WINDOWS_OUT=/Volumes/jsteele/Downloads/dockpipe.exe
# (exact /Volumes/… name appears in Finder after connecting to smb://anton.local/…)
WINDOWS_OUT ?= bin/dockpipe.exe
.PHONY: build build-windows test check-paths deb deb-all
build:
	cp VERSION cmd/dockpipe/VERSION
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/dockpipe.bin ./cmd/dockpipe
	@echo "Built bin/dockpipe.bin — run via bin/dockpipe"

# Cross-compile for Windows (default bin/dockpipe.exe — gitignored). From repo root only.
build-windows:
	cp VERSION cmd/dockpipe/VERSION
	@mkdir -p "$(dir $(WINDOWS_OUT))"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)" -o "$(WINDOWS_OUT)" ./cmd/dockpipe
	@echo "Built $(WINDOWS_OUT) — copy to your Windows machine or PATH"

test:
	go test ./...

# Docs/code guardrail: obsolete templates/core paths (pre-assets layout). See CONTRIBUTING.md.
check-paths:
	bash scripts/check-templates-core-paths.sh

# Debian packages (requires dpkg-deb). Default arch amd64; deb-all builds amd64 + arm64.
deb:
	./packaging/build-deb.sh "$(DEB_VERSION)" amd64

deb-all:
	./packaging/build-deb-all.sh "$(DEB_VERSION)"
