# Build the Go CLI into bin/dockpipe.bin (bin/dockpipe launcher invokes it).
# build: compile only — does not install. Use: make install (needs prior build) or make dev-install (build + install).
# Keep in sync with repo-root VERSION (used by CI / release).
DEB_VERSION ?= $(shell test -f VERSION && tr -d ' \t\r\n' < VERSION || echo 0.6.0)
GO_LDFLAGS := -s -w -X main.Version=$(DEB_VERSION)
# Windows exe output path. smb://… is not a filesystem path—mount the share in Finder / Files first, then use the mount path, e.g.
#   make build-windows WINDOWS_OUT=/Volumes/jsteele/Downloads/dockpipe.exe
# (exact /Volumes/… name appears in Finder after connecting to smb://anton.local/…)
WINDOWS_OUT ?= bin/dockpipe.exe
.PHONY: build build-windows build-code-server-image pipeon-icons install-pipeon-shortcut install-pipeon-shortcut-windows install-pipeon-shortcut-macos install dev-install test test-quick check-paths deb deb-all demo-record demo-record-short demo-record-long dev-deps install-record-deps ci self-analysis self-analysis-host self-analysis-stack compliance-handoff user-insight-process pipeon-status pipeon-bundle pipeon-chat
build:
	cp VERSION cmd/dockpipe/VERSION
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/dockpipe.bin ./cmd/dockpipe
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/dorkpipe ./cmd/dorkpipe
	@echo "Built bin/dockpipe.bin — run via bin/dockpipe (repo) or make install / make dev-install"
	@echo "Built bin/dorkpipe (DorkPipe DAG orchestrator on lib/dorkpipe)"

# Install pre-built binary to a local PATH directory (~/.local/bin, %USERPROFILE%\\bin, …). Does not run go build.
install:
	bash scripts/install-dockpipe.sh

# Developer loop: compile then install (same as: make build && make install).
dev-install: build install

# Cross-compile for Windows (default bin/dockpipe.exe — gitignored). From repo root only.
build-windows:
	cp VERSION cmd/dockpipe/VERSION
	@mkdir -p "$(dir $(WINDOWS_OUT))"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)" -o "$(WINDOWS_OUT)" ./cmd/dockpipe
	@echo "Built $(WINDOWS_OUT) — copy to your Windows machine or PATH"

# Regenerate Pipeon P-mark PNG / favicon.ico / SVG (requires Pillow: pip install Pillow).
pipeon-icons:
	python3 scripts/pipeon/generate-pipeon-icons.py

# Pipeon shortcuts with P icon: Linux (Freedesktop), macOS (~/Applications/Pipeon.command), Windows (.lnk).
# From Git Bash on Windows, `make install-pipeon-shortcut` runs the PowerShell installer.
install-pipeon-shortcut:
	@UNAME="$$(uname -s 2>/dev/null || echo unknown)"; \
	case "$$UNAME" in \
	  Darwin) bash scripts/pipeon/install-pipeon-shortcut-macos.sh ;; \
	  Linux) bash scripts/pipeon/install-pipeon-desktop-shortcut.sh ;; \
	  MINGW*|MSYS*|CYGWIN*) powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/pipeon/install-pipeon-desktop-shortcut.ps1 ;; \
	  *) echo "Unknown OS (uname=$$UNAME). Try: make install-pipeon-shortcut-macos | install-pipeon-shortcut-windows, or run scripts under scripts/pipeon/ manually." >&2; exit 1 ;; \
	esac

install-pipeon-shortcut-windows:
	powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/pipeon/install-pipeon-desktop-shortcut.ps1

install-pipeon-shortcut-macos:
	bash scripts/pipeon/install-pipeon-shortcut-macos.sh

# Coder code-server image with Pipeon extension (workflow vscode). Requires Docker; build from repo root.
build-code-server-image:
	docker build -t dockpipe-code-server:latest -f templates/core/resolvers/code-server/assets/images/code-server/Dockerfile .

test:
	go test ./...

# Go + template guard + bash unit tests (no Docker). Faster than full CI.
test-quick:
	bash scripts/test-quick.sh

# Same sequence as Linux job in .github/workflows/ci.yml (not CodeQL, not Windows).
ci:
	bash scripts/ci-local.sh

# Docs/code guardrail: obsolete templates/core paths (pre-assets layout). See CONTRIBUTING.md.
check-paths:
	bash scripts/check-templates-core-paths.sh

# Debian packages (requires dpkg-deb). Default arch amd64; deb-all builds amd64 + arm64.
deb:
	./packaging/build-deb.sh "$(DEB_VERSION)" amd64

deb-all:
	./packaging/build-deb-all.sh "$(DEB_VERSION)"

# Terminal demo GIFs for sharing (requires asciinema + agg + Docker — see demo/README.md).
demo-record: build
	bash scripts/record-demo.sh all

demo-record-short: build
	bash scripts/record-demo.sh short

demo-record-long: build
	bash scripts/record-demo.sh long

# Optional dev tools: CI (govulncheck, gosec) + best-effort demo-record (asciinema, agg). Not for end users.
dev-deps:
	bash scripts/install-deps.sh
	bash scripts/install-record-deps.sh

# Optional tools for `make demo-record` only (see demo/README.md).
install-record-deps:
	bash scripts/install-record-deps.sh

# Accelerator: DorkPipe self-analysis in this repo (Cursor handoff + paste prompt). Requires Docker for default targets.
# make self-analysis-host — same DAG on the host if you have no Docker.
# DORKPIPE_DEV_STACK_AUTODOWN=0 make self-analysis-stack — leave Postgres+Ollama up after.
self-analysis: build
	./bin/dockpipe --workflow dorkpipe-self-analysis --workdir . --

self-analysis-host: build
	./bin/dockpipe --workflow dorkpipe-self-analysis-host --workdir . --

self-analysis-stack: build
	./bin/dockpipe --workflow dorkpipe-self-analysis-stack --workdir . --

# AI / governance: print compliance & security signal paths (host workflow; see docs/compliance-ai-handoff.md).
compliance-handoff: build
	./bin/dockpipe --workflow compliance-handoff --workdir . --

# User insight queue: normalize queue.json → insights.json + by-category (host workflow; see docs/user-insight-queue.md).
user-insight-process: build
	./bin/dockpipe --workflow user-insight-process --workdir . --

# Pipeon: local Ollama context + chat helpers (feature-flagged; see scripts/pipeon/README.md). Use PROMPT='...' for chat.
pipeon-status:
	DOCKPIPE_PIPEON=1 DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1 ./bin/pipeon status

pipeon-bundle:
	DOCKPIPE_PIPEON=1 DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1 ./bin/pipeon bundle

pipeon-chat:
	@test -n "$(PROMPT)" || (echo 'Usage: make pipeon-chat PROMPT="your question"'; exit 1)
	DOCKPIPE_PIPEON=1 DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1 ./bin/pipeon chat "$(PROMPT)"
