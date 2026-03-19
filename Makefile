# Build the Go CLI into bin/dockpipe.bin (bin/dockpipe launcher invokes it).
# Keep in sync with repo-root VERSION (used by CI / release).
DEB_VERSION ?= $(shell test -f VERSION && tr -d ' \t\r\n' < VERSION || echo 0.5.8)
.PHONY: build test deb deb-all
build:
	go build -trimpath -ldflags "-s -w" -o bin/dockpipe.bin ./cmd/dockpipe
	@echo "Built bin/dockpipe.bin — run via bin/dockpipe"

test:
	go test ./...

# Debian packages (requires dpkg-deb). Default arch amd64; deb-all builds amd64 + arm64.
deb:
	./packaging/build-deb.sh "$(DEB_VERSION)" amd64

deb-all:
	./packaging/build-deb-all.sh "$(DEB_VERSION)"
