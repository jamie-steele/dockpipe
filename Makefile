# Repository Makefile — Go build rules live in src/Makefile (run `make` from repo root).
#
# Use the product: after `make build`, run workflows with DockPipe (compiled packages resolve like any project):
#   ./src/bin/dockpipe --workflow <name> --workdir . --
#
# Optional: `make maintainer-tools` — dorkpipe + mcpd binaries under packages/
# Optional Pipeon: `make pipeon-icons` | `make build-code-server-image` (see packages/pipeon/resolvers/pipeon/vscode-extension/)
# Qt launcher: cmake -S src/apps/pipeon-launcher -B src/apps/pipeon-launcher/build && cmake --build ...
include src/Makefile

.PHONY: pipeon-icons build-code-server-image install dev-install test-quick check-paths deb deb-all demo-record demo-record-short demo-record-long dev-deps install-record-deps ci package-templates-core package-dockpipe-language-support package-vscode-language-support install-dockpipe-language-support package-pipeon-vscode-extension install-pipeon-vscode-extension

pipeon-icons:
	python3 packages/pipeon/resolvers/pipeon/assets/scripts/generate-pipeon-icons.py

build-code-server-image:
	docker build -t dockpipe-code-server:latest -f packages/pipeon/resolvers/pipeon/vscode-extension/Dockerfile.code-server .

# Package Pipeon VS Code extension (.vsix) into bin/.dockpipe/extensions.
package-pipeon-vscode-extension:
	mkdir -p bin/.dockpipe/extensions
	cd packages/pipeon/resolvers/pipeon/vscode-extension && npx --yes @vscode/vsce package -o ../../../../../bin/.dockpipe/extensions/pipeon-$$(node -p "require('./package.json').version").vsix

# Build + install Pipeon extension into Cursor (fallback: VS Code CLI).
install-pipeon-vscode-extension: package-pipeon-vscode-extension
	VSIX="$$(ls -1t bin/.dockpipe/extensions/pipeon-*.vsix | head -n1)"; \
	INSTALLED=0; \
	if command -v cursor >/dev/null 2>&1; then \
	  echo "[dockpipe] installing Pipeon into Cursor: $$VSIX"; \
	  cursor --install-extension "$$VSIX"; \
	  INSTALLED=1; \
	fi; \
	if command -v code >/dev/null 2>&1; then \
	  echo "[dockpipe] installing Pipeon into VS Code: $$VSIX"; \
	  code --install-extension "$$VSIX"; \
	  INSTALLED=1; \
	fi; \
	if [ "$$INSTALLED" -eq 0 ]; then \
	  echo "[dockpipe] no editor CLI found. Install manually from VSIX: $$VSIX"; \
	fi

# Package DockPipe language support extension (.vsix).
package-dockpipe-language-support:
	mkdir -p bin/.dockpipe/extensions
	cd packages/dockpipe-language-support && npm install && npx --yes @vscode/vsce package -o ../../bin/.dockpipe/extensions/dockpipe-language-support-$$(node -p "require('./package.json').version").vsix

# Back-compat alias.
package-vscode-language-support: package-dockpipe-language-support

# Build + install DockPipe language support into Cursor (fallback: VS Code CLI).
install-dockpipe-language-support: package-dockpipe-language-support
	VSIX="$$(ls -1t bin/.dockpipe/extensions/dockpipe-language-support-*.vsix | head -n1)"; \
	INSTALLED=0; \
	if command -v cursor >/dev/null 2>&1; then \
	  echo "[dockpipe] installing DockPipe language support into Cursor: $$VSIX"; \
	  cursor --install-extension "$$VSIX"; \
	  INSTALLED=1; \
	fi; \
	if command -v code >/dev/null 2>&1; then \
	  echo "[dockpipe] installing DockPipe language support into VS Code: $$VSIX"; \
	  code --install-extension "$$VSIX"; \
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
