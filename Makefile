# Repository Makefile — Go build rules live in src/Makefile (run `make` from repo root).
include src/Makefile

.PHONY: build-code-server-image pipeon-icons pipeon-launcher install-pipeon-shortcut install-pipeon-launcher-shortcut install-pipeon-all-shortcuts install-pipeon-shortcut-windows install-pipeon-shortcut-macos install dev-install test-quick check-paths deb deb-all demo-record demo-record-short demo-record-long dev-deps install-record-deps ci self-analysis self-analysis-host self-analysis-stack compliance-handoff dockpipe.cloudflare.r2publish r2-publish package-templates-core user-insight-process pipeon-status pipeon-bundle pipeon-chat

# Install pre-built binary to a local PATH directory (~/.local/bin, %USERPROFILE%\\bin, …). Does not run go build.
install:
	bash src/scripts/install-dockpipe.sh

# Developer loop: compile then install (same as: make build && make install).
dev-install: build install

# Regenerate Pipeon P-mark PNG / favicon.ico / SVG (requires Pillow: pip install Pillow).
pipeon-icons:
	python3 src/apps/pipeon/scripts/generate-pipeon-icons.py

# Qt Pipeon Launcher — CMake build directory is src/apps/pipeon-launcher/build (see src/apps/pipeon-launcher/README.md).
pipeon-launcher:
	cmake -S src/apps/pipeon-launcher -B src/apps/pipeon-launcher/build -DCMAKE_BUILD_TYPE=Release
	cmake --build src/apps/pipeon-launcher/build -j$$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)

# Pipeon shortcuts with P icon: Linux (Freedesktop), macOS (~/Applications/Pipeon.command), Windows (.lnk).
# `install-pipeon-shortcut` = code-server in browser (dockpipe --workflow vscode). NOT the Qt tray app.
# Qt tray: `make pipeon-launcher` then `make install-pipeon-launcher-shortcut` (Linux Freedesktop).
# From Git Bash on Windows, `make install-pipeon-shortcut` runs the PowerShell installer.
install-pipeon-shortcut:
	@UNAME="$$(uname -s 2>/dev/null || echo unknown)"; \
	case "$$UNAME" in \
	  Darwin) bash src/apps/pipeon/scripts/install-pipeon-shortcut-macos.sh ;; \
	  Linux) bash src/apps/pipeon/scripts/install-pipeon-desktop-shortcut.sh ;; \
	  MINGW*|MSYS*|CYGWIN*) powershell.exe -NoProfile -ExecutionPolicy Bypass -File src/apps/pipeon/scripts/install-pipeon-desktop-shortcut.ps1 ;; \
	  *) echo "Unknown OS (uname=$$UNAME). Try: make install-pipeon-shortcut-macos | install-pipeon-shortcut-windows, or run scripts under src/apps/pipeon/scripts/ manually." >&2; exit 1 ;; \
	esac

install-pipeon-shortcut-windows:
	powershell.exe -NoProfile -ExecutionPolicy Bypass -File src/apps/pipeon/scripts/install-pipeon-desktop-shortcut.ps1

install-pipeon-shortcut-macos:
	bash src/apps/pipeon/scripts/install-pipeon-shortcut-macos.sh

# Linux (Pop!_OS, etc.): ~/.local/share/applications/pipeon-launcher.desktop — requires `make pipeon-launcher` first.
install-pipeon-launcher-shortcut:
	bash src/apps/pipeon/scripts/install-pipeon-launcher-desktop-shortcut.sh

# Linux: Qt launcher shortcut + code-server shortcut. Other OS: use install-pipeon-shortcut only.
install-pipeon-all-shortcuts:
	@UNAME="$$(uname -s 2>/dev/null || echo unknown)"; \
	case "$$UNAME" in \
	  Linux) $(MAKE) pipeon-launcher install-pipeon-launcher-shortcut install-pipeon-shortcut ;; \
	  *) echo "install-pipeon-all-shortcuts is Linux-only (Freedesktop). Try: make install-pipeon-shortcut" >&2; exit 1 ;; \
	esac

# Coder code-server image with Pipeon extension (workflow vscode). Requires Docker; build from repo root.
build-code-server-image:
	docker build -t dockpipe-code-server:latest -f src/contrib/pipeon-vscode-extension/Dockerfile.code-server .

# Go + template guard + bash unit tests (no Docker). Faster than full CI.
test-quick:
	bash src/scripts/test-quick.sh

# Same sequence as Linux job in .github/workflows/ci.yml (not CodeQL, not Windows).
ci:
	bash src/scripts/ci-local.sh

# Tar.gz + sha256 + install-manifest.json for `dockpipe install core` (upload release/artifacts/* to your HTTPS base URL).
package-templates-core:
	bash release/packaging/package-templates-core.sh

# Docs/code guardrail: obsolete templates/core paths (pre-assets layout). See CONTRIBUTING.md.
check-paths:
	bash src/scripts/check-templates-core-paths.sh

# Debian packages (requires dpkg-deb). Default arch amd64; deb-all builds amd64 + arm64.
deb:
	./release/packaging/build-deb.sh "$(DEB_VERSION)" amd64

deb-all:
	./release/packaging/build-deb-all.sh "$(DEB_VERSION)"

# Terminal demo GIFs for sharing (requires asciinema + agg + Docker — see release/demo/README.md).
demo-record: build
	bash src/scripts/record-demo.sh all

demo-record-short: build
	bash src/scripts/record-demo.sh short

demo-record-long: build
	bash src/scripts/record-demo.sh long

# Optional dev tools: CI (govulncheck, gosec) + best-effort demo-record (asciinema, agg). Not for end users.
dev-deps:
	bash src/scripts/install-deps.sh
	bash src/scripts/install-record-deps.sh

# Optional tools for `make demo-record` only (see release/demo/README.md).
install-record-deps:
	bash src/scripts/install-record-deps.sh

# Accelerator: DorkPipe self-analysis in this repo (Cursor handoff + paste prompt). Requires Docker for default targets.
# make self-analysis-host — same DAG on the host if you have no Docker.
# DORKPIPE_DEV_STACK_AUTODOWN=0 make self-analysis-stack — leave Postgres+Ollama up after.
self-analysis: build
	./src/bin/dockpipe --workflow dorkpipe-self-analysis --workdir . --

self-analysis-host: build
	./src/bin/dockpipe --workflow dorkpipe-self-analysis-host --workdir . --

self-analysis-stack: build
	./src/bin/dockpipe --workflow dorkpipe-self-analysis-stack --workdir . --

# AI / governance: print compliance & security signal paths (host workflow; see docs/compliance-ai-handoff.md).
compliance-handoff: build
	./src/bin/dockpipe --workflow compliance-handoff --workdir . --

# Dogfood: tar ./release/artifacts and upload to Cloudflare R2 (S3 API). Set R2_BUCKET, CLOUDFLARE_ACCOUNT_ID, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY.
dockpipe.cloudflare.r2publish: build
	./src/bin/dockpipe --workflow dockpipe.cloudflare.r2publish --workdir . --

# Back-compat alias for muscle memory / scripts
r2-publish: dockpipe.cloudflare.r2publish

# User insight queue: normalize queue.json → insights.json + by-category (host workflow; see docs/user-insight-queue.md).
user-insight-process: build
	./src/bin/dockpipe --workflow user-insight-process --workdir . --

# Pipeon: local Ollama context + chat helpers (feature-flagged; see src/apps/pipeon/scripts/README.md). Use PROMPT='...' for chat.
pipeon-status:
	DOCKPIPE_PIPEON=1 DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1 ./src/bin/pipeon status

pipeon-bundle:
	DOCKPIPE_PIPEON=1 DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1 ./src/bin/pipeon bundle

pipeon-chat:
	@test -n "$(PROMPT)" || (echo 'Usage: make pipeon-chat PROMPT="your question"'; exit 1)
	DOCKPIPE_PIPEON=1 DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1 ./src/bin/pipeon chat "$(PROMPT)"
