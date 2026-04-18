# Repository Makefile — Go build rules live in src/Makefile (run `make` from repo root).
#
# Use the product: after `make build`, run workflows with DockPipe (compiled packages resolve like any project):
#   ./src/bin/dockpipe --workflow <name> --workdir . --
#
# Optional: `make maintainer-tools` — dorkpipe + mcpd binaries under packages/
# Optional Pipeon: `make pipeon-icons` | `make build-code-server-image` (see packages/pipeon/resolvers/pipeon/vscode-extension/)
# DockPipe Launcher: cmake -S src/app/tooling/dockpipe-launcher -B src/app/tooling/dockpipe-launcher/build && cmake --build ...
include src/Makefile

.PHONY: pipeon-icons build-code-server-image build-pipeon-desktop build-dockpipe-launcher install-pipeon-desktop install-pipeon-desktop-global install-dockpipe-launcher install-dockpipe-launcher-global install dev-install test-quick check-paths deb deb-all demo-record demo-record-short demo-record-long dev-deps install-record-deps ci package-templates-core package-dockpipe-language-support package-vscode-language-support install-dockpipe-language-support package-pipeon-vscode-extension install-pipeon-vscode-extension

pipeon-icons:
	python3 packages/pipeon/resolvers/pipeon/assets/scripts/generate-pipeon-icons.py

build-code-server-image: package-dockpipe-language-support package-pipeon-vscode-extension
	docker build -t dockpipe-code-server:latest -f packages/pipeon/resolvers/pipeon/vscode-extension/Dockerfile.code-server .

build-pipeon-desktop:
	cargo build --manifest-path packages/pipeon/apps/pipeon-desktop/src-tauri/Cargo.toml --release
	mkdir -p packages/pipeon/apps/pipeon-desktop/bin
	cp -f packages/pipeon/apps/pipeon-desktop/src-tauri/target/release/pipeon-desktop packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop
	chmod +x packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop

build-dockpipe-launcher:
	rm -rf src/app/tooling/dockpipe-launcher/build
	cmake -S src/app/tooling/dockpipe-launcher -B src/app/tooling/dockpipe-launcher/build
	cmake --build src/app/tooling/dockpipe-launcher/build

install-pipeon-desktop: build-pipeon-desktop
	mkdir -p bin/.dockpipe/packages/pipeon/bin
	install -m 755 packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop bin/.dockpipe/packages/pipeon/bin/pipeon-desktop

install-pipeon-desktop-global: install-pipeon-desktop
	mkdir -p "$$HOME/.local/share/pipeon/bin"
	mkdir -p "$$HOME/.local/share/pipeon/icons"
	mkdir -p "$$HOME/.local/share/applications"
	install -m 755 bin/.dockpipe/packages/pipeon/bin/pipeon-desktop "$$HOME/.local/share/pipeon/bin/pipeon-desktop"
	install -m 644 packages/pipeon/apps/pipeon-desktop/src-tauri/icons/icon.png "$$HOME/.local/share/pipeon/icons/pipeon.png"
	rm -f "$$HOME/.local/share/applications/com.dockpipe.pipeon.desktop"
	printf '%s\n' \
		'[Desktop Entry]' \
		'Type=Application' \
		'Name=Pipeon' \
		'Exec='"$$HOME"'/.local/share/pipeon/bin/pipeon-desktop' \
		'Icon='"$$HOME"'/.local/share/pipeon/icons/pipeon.png' \
		'Terminal=false' \
		'Categories=Development;' \
		'StartupNotify=true' \
		'StartupWMClass=com.dockpipe.pipeon' \
		> "$$HOME/.local/share/applications/com.dockpipe.pipeon.desktop"

install-dockpipe-launcher: build-dockpipe-launcher
	mkdir -p bin/.dockpipe/tooling/bin
	install -m 755 src/app/tooling/dockpipe-launcher/build/dockpipe-launcher bin/.dockpipe/tooling/bin/dockpipe-launcher

install-dockpipe-launcher-global: install-dockpipe-launcher
	mkdir -p "$$HOME/.local/share/dockpipe/bin"
	mkdir -p "$$HOME/.local/share/dockpipe/icons"
	mkdir -p "$$HOME/.local/share/applications"
	install -m 755 bin/.dockpipe/tooling/bin/dockpipe-launcher "$$HOME/.local/share/dockpipe/bin/dockpipe-launcher"
	install -m 644 packages/dorkpipe/assets/images/icon.png "$$HOME/.local/share/dockpipe/icons/dockpipe.png"
	rm -f "$$HOME/.local/share/applications/dockpipe-launcher.desktop"
	printf '%s\n' \
		'[Desktop Entry]' \
		'Type=Application' \
		'Name=DockPipe Launcher' \
		'Exec='"$$HOME"'/.local/share/dockpipe/bin/dockpipe-launcher --start-home' \
		'Icon='"$$HOME"'/.local/share/dockpipe/icons/dockpipe.png' \
		'Terminal=false' \
		'Categories=Development;' \
		'StartupNotify=true' \
		'StartupWMClass=dockpipe-launcher' \
		> "$$HOME/.local/share/applications/dockpipe-launcher.desktop"

# Package Pipeon VS Code extension (.vsix) into bin/.dockpipe/extensions.
# Reuses the locally installed vsce from the DockPipe language-support extension to avoid network fetches.
package-pipeon-vscode-extension: package-dockpipe-language-support
	mkdir -p bin/.dockpipe/extensions
	npm --prefix packages/pipeon/resolvers/pipeon/vscode-extension run build
	npm --prefix packages/pipeon/resolvers/pipeon/vscode-extension run test:webview
	cd packages/pipeon/resolvers/pipeon/vscode-extension && node ../../../../../src/app/tooling/vscode-extensions/dockpipe-language-support/node_modules/@vscode/vsce/vsce package --no-dependencies -o ../../../../../bin/.dockpipe/extensions/pipeon-$$(node -p "require('./package.json').version").vsix

# Build + install Pipeon extension into Cursor (fallback: VS Code CLI).
install-pipeon-vscode-extension: package-pipeon-vscode-extension
	VSIX="$$(ls -1t bin/.dockpipe/extensions/pipeon-*.vsix | head -n1)"; \
	INSTALLED=0; \
	if command -v cursor >/dev/null 2>&1; then \
	  echo "[dockpipe] installing Pipeon into Cursor: $$VSIX"; \
	  cursor --install-extension "$$VSIX" --force; \
	  INSTALLED=1; \
	fi; \
	if command -v code >/dev/null 2>&1; then \
	  echo "[dockpipe] installing Pipeon into VS Code: $$VSIX"; \
	  code --install-extension "$$VSIX" --force; \
	  INSTALLED=1; \
	fi; \
	if [ "$$INSTALLED" -eq 0 ]; then \
	  echo "[dockpipe] no editor CLI found. Install manually from VSIX: $$VSIX"; \
	fi

# Package DockPipe language support extension (.vsix).
package-dockpipe-language-support:
	mkdir -p bin/.dockpipe/extensions
	cd src/app/tooling/vscode-extensions/dockpipe-language-support && if [ ! -x node_modules/.bin/vsce ]; then NPM_CONFIG_CACHE=$$(pwd)/../../../../tmp/npm-cache npm ci --no-audit --no-fund; fi && NPM_CONFIG_CACHE=$$(pwd)/../../../../tmp/npm-cache node node_modules/@vscode/vsce/vsce package --no-dependencies -o ../../../../bin/.dockpipe/extensions/dockpipe-language-support-$$(node -p "require('./package.json').version").vsix

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
