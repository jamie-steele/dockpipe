# Repository maintainer Makefile.
#
# This file is a convenience layer for working inside the DockPipe source checkout.
# It is not the package/runtime contract itself. Product/package lifecycle behavior
# should live behind DockPipe commands such as:
#   dockpipe build
#   dockpipe package build
#
# Main local dev entrypoint:
#   make build
#     Builds DockPipe core + DockPipe Launcher for this checkout.
#
# Contributors who want plain `dockpipe ...` on PATH should run:
#   make dev-install
include src/Makefile

build: build-dockpipe-launcher

.PHONY: \
	maintainer-tools test \
	build-dockpipe-launcher install-dockpipe-launcher install-dockpipe-launcher-global \
	install dev-install test-quick check-paths deb deb-all demo-record demo-record-short demo-record-long \
	dev-deps install-record-deps ci package-templates-core package-dockpipe-language-support \
	package-vscode-language-support install-dockpipe-language-support

build-dockpipe-launcher:
	rm -rf src/app/tooling/dockpipe-launcher/build
	cmake -S src/app/tooling/dockpipe-launcher -B src/app/tooling/dockpipe-launcher/build
	cmake --build src/app/tooling/dockpipe-launcher/build

# Deprecated maintainer alias at the repo layer only. Package-owned source builds live behind
# dockpipe package build for source checkouts.
maintainer-tools: build
	./src/bin/dockpipe package build --workdir .

# Repo test sweep: core Go tests plus DockPipe-owned package/workflow test hooks.
test: build
	go test ./...
	./src/bin/dockpipe test --workdir .

install-dockpipe-launcher: build-dockpipe-launcher
	mkdir -p bin/.dockpipe/tooling/bin
	mkdir -p bin/.dockpipe/tooling/share/icons/hicolor
	mkdir -p bin/.dockpipe/tooling/share/icons
	install -m 755 src/app/tooling/dockpipe-launcher/build/dockpipe-launcher bin/.dockpipe/tooling/bin/dockpipe-launcher
	cp -R src/app/tooling/dockpipe-launcher/resources/icons/hicolor/. bin/.dockpipe/tooling/share/icons/hicolor/
	install -m 644 src/app/tooling/dockpipe-launcher/resources/images/dockpipe-launcher.png bin/.dockpipe/tooling/share/icons/dockpipe-launcher.png

# Internal/local desktop helper: installs the repo-built DockPipe Launcher under ~/.local/share.
install-dockpipe-launcher-global: install-dockpipe-launcher
	mkdir -p "$$HOME/.local/share/dockpipe/bin"
	mkdir -p "$$HOME/.local/share/dockpipe/icons"
	mkdir -p "$$HOME/.local/share/icons/hicolor"
	mkdir -p "$$HOME/.local/share/applications"
	install -m 755 bin/.dockpipe/tooling/bin/dockpipe-launcher "$$HOME/.local/share/dockpipe/bin/dockpipe-launcher"
	install -m 644 bin/.dockpipe/tooling/share/icons/dockpipe-launcher.png "$$HOME/.local/share/dockpipe/icons/dockpipe-launcher.png"
	cp -R bin/.dockpipe/tooling/share/icons/hicolor/. "$$HOME/.local/share/icons/hicolor/"
	rm -f "$$HOME/.local/share/applications/dockpipe-launcher.desktop"
	printf '%s\n' \
		'[Desktop Entry]' \
		'Type=Application' \
		'Name=DockPipe Launcher' \
		'Exec=/usr/bin/env -u DESKTOP_STARTUP_ID -u XDG_ACTIVATION_TOKEN '"$$HOME"'/.local/share/dockpipe/bin/dockpipe-launcher --start-home' \
		'Icon=dockpipe-launcher' \
		'Terminal=false' \
		'Categories=Development;' \
		'StartupNotify=false' \
		'X-GNOME-WMClass=dockpipe-launcher' \
		'StartupWMClass=dockpipe-launcher' \
		> "$$HOME/.local/share/applications/dockpipe-launcher.desktop"
	if command -v update-desktop-database >/dev/null 2>&1; then update-desktop-database "$$HOME/.local/share/applications" >/dev/null 2>&1 || true; fi

# Package DockPipe language support extension (.vsix).
package-dockpipe-language-support:
	mkdir -p bin/.dockpipe/extensions
	cd src/app/tooling/vscode-extensions/dockpipe-language-support && if [ ! -x node_modules/.bin/vsce ]; then NPM_CONFIG_CACHE=$$(pwd)/../../../../tmp/npm-cache npm ci --no-audit --no-fund; fi && NPM_CONFIG_CACHE=$$(pwd)/../../../../tmp/npm-cache node node_modules/@vscode/vsce/vsce package --no-dependencies -o ../../../../../bin/.dockpipe/extensions/dockpipe-language-support-$$(node -p "require('./package.json').version").vsix

# Back-compat alias.
package-vscode-language-support: package-dockpipe-language-support

# Build + install DockPipe language support into Cursor (fallback: VS Code CLI).
install-dockpipe-language-support: package-dockpipe-language-support
	VSIX="$$(ls -1t bin/.dockpipe/extensions/dockpipe-language-support-*.vsix | head -n1)"; \
	INSTALLED=0; \
	if command -v cursor >/dev/null 2>&1; then \
	  echo "[dockpipe] installing DockPipe language support into Cursor: $$VSIX"; \
	  cursor --install-extension "$$VSIX" --force; \
	  INSTALLED=1; \
	fi; \
	if command -v code >/dev/null 2>&1; then \
	  echo "[dockpipe] installing DockPipe language support into VS Code: $$VSIX"; \
	  code --install-extension "$$VSIX" --force; \
	  INSTALLED=1; \
	fi; \
	if [ "$$INSTALLED" -eq 0 ]; then \
	  echo "[dockpipe] no editor CLI found. Install manually from VSIX: $$VSIX"; \
	fi

# Install pre-built binary to a local PATH directory (~/.local/bin, %USERPROFILE%\\bin, …). Does not run go build.
install:
	bash src/scripts/install-dockpipe.sh

# Developer loop: compile then install (same as: make build && make install).
dev-install: build install

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
