#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_ROOT="$(cd "${DOCKPIPE_PACKAGE_ROOT:-$SCRIPT_DIR/../..}" && pwd)"
REPO_ROOT="$(cd "${DOCKPIPE_WORKDIR:-$PACKAGE_ROOT/../..}" && pwd)"
SDK_SH="$REPO_ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh"

if [[ -f "$SDK_SH" ]]; then
  # shellcheck source=/dev/null
  source "$SDK_SH"
  dockpipe_sdk_refresh "$REPO_ROOT"
  PACKAGE_ROOT="$(dockpipe get package_root 2>/dev/null || printf '%s\n' "$PACKAGE_ROOT")"
  REPO_ROOT="$(dockpipe get workdir 2>/dev/null || printf '%s\n' "$REPO_ROOT")"
fi

PIPEON_DESKTOP_TARGET_DIR="${PIPEON_DESKTOP_TARGET_DIR:-$REPO_ROOT/bin/.dockpipe/build/pipeon-desktop-target}"
PIPEON_VSCODE_EXT_SRC="$REPO_ROOT/packages/pipeon/resolvers/pipeon/vscode-extension"
PIPEON_VSCODE_EXT_BUILD_DIR="${PIPEON_VSCODE_EXT_BUILD_DIR:-$REPO_ROOT/bin/.dockpipe/build/pipeon-vscode-extension}"
PIPEON_VSCODE_EXT_NPM_CACHE="${PIPEON_VSCODE_EXT_NPM_CACHE:-$REPO_ROOT/bin/.dockpipe/build/npm-cache}"
DOCKPIPE_VSCODE_EXT_DIR="$REPO_ROOT/src/app/tooling/vscode-extensions/dockpipe-language-support"
DOCKPIPE_VSCODE_TMP_CACHE="$REPO_ROOT/tmp/npm-cache"

usage() {
  cat <<'EOF'
Pipeon package build helper

Usage:
  packages/pipeon/assets/scripts/build.sh <command>

Commands:
  source                   Build Pipeon's package-owned source artifacts for repo/dev use
  icons                    Regenerate Pipeon icon assets
  desktop                  Build the Pipeon desktop binary
  install-desktop-global   Install the repo-built Pipeon desktop under ~/.local/share
  dockpipe-language-support Package the DockPipe language support VSIX dependency
  vscode-extension         Package the Pipeon VSIX
  install-vscode-extension Install the packaged Pipeon VSIX into Cursor / VS Code when available
  code-server-image        Build the branded dockpipe-code-server image
EOF
}

prompt_install_host_tool() {
  local prompt_id="$1" title="$2" message="$3" default_value="${4:-no}" intent="${5:-host-mutation}" automation_group="${6:-host-tools}" auto_approve_value="${7:-yes}"
  if declare -F dockpipe_sdk >/dev/null 2>&1; then
    dockpipe_sdk prompt confirm \
      --id "$prompt_id" \
      --title "$title" \
      --message "$message" \
      --default "$default_value" \
      --intent "$intent" \
      --automation-group "$automation_group" \
      --allow-auto-approve \
      --auto-approve-value "$auto_approve_value"
    return $?
  fi
  printf '%s [y/N] ' "$message" >&2
  local reply
  IFS= read -r reply || return 1
  case "${reply,,}" in
    y|yes) printf 'yes\n' ;;
    *) printf 'no\n' ;;
  esac
}

prompt_install_cargo() {
  prompt_install_host_tool \
    "pipeon.install-cargo" \
    "Install Rust Toolchain?" \
    "Pipeon source build needs Cargo/Rust before it can build the desktop app. Allow DockPipe to launch the install command for this host?" \
    no \
    host-mutation \
    pipeon-host-tools \
    yes
}

install_cargo_windows() {
  if command -v winget >/dev/null 2>&1; then
    echo "[dockpipe] installing Rust via winget (Rustup)..."
    winget install --id Rustlang.Rustup --exact
    return $?
  fi
  echo "[dockpipe] winget is not available. Install Rust manually from https://rustup.rs/ and rerun." >&2
  return 1
}

install_cargo_host() {
  case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*)
      install_cargo_windows
      ;;
    Linux)
      if command -v apt-get >/dev/null 2>&1; then
        echo "[dockpipe] run: curl https://sh.rustup.rs -sSf | sh" >&2
      elif command -v dnf >/dev/null 2>&1; then
        echo "[dockpipe] run: sudo dnf install -y cargo rustup" >&2
      elif command -v pacman >/dev/null 2>&1; then
        echo "[dockpipe] run: sudo pacman -S --needed rustup" >&2
      else
        echo "[dockpipe] install Rust manually from https://rustup.rs/ and rerun." >&2
      fi
      return 1
      ;;
    *)
      echo "[dockpipe] install Rust manually from https://rustup.rs/ and rerun." >&2
      return 1
      ;;
  esac
}

require_cargo() {
  if command -v cargo >/dev/null 2>&1; then
    return 0
  fi
  local answer
  answer="$(prompt_install_cargo)" || answer="no"
  if [[ "$answer" == "yes" ]]; then
    if install_cargo_host; then
      echo "[dockpipe] Cargo install command finished. Open a new shell if needed, then rerun the Pipeon source build." >&2
      exit 1
    fi
    echo "[dockpipe] Cargo is still unavailable. Finish the Rust install, open a new shell, and rerun." >&2
    exit 1
  fi
  echo "[dockpipe] Pipeon source build requires Cargo. Install Rust/Cargo and rerun." >&2
  exit 1
}

build_source() {
  build_desktop
  package_vscode_extension
}

package_dockpipe_language_support() {
  mkdir -p "$REPO_ROOT/bin/.dockpipe/extensions"
  (
    cd "$DOCKPIPE_VSCODE_EXT_DIR"
    if [[ ! -x node_modules/.bin/vsce ]]; then
      NPM_CONFIG_CACHE="$DOCKPIPE_VSCODE_TMP_CACHE" npm ci --no-audit --no-fund
    fi
    NPM_CONFIG_CACHE="$DOCKPIPE_VSCODE_TMP_CACHE" \
      node node_modules/@vscode/vsce/vsce package --no-dependencies \
      -o "$REPO_ROOT/bin/.dockpipe/extensions/dockpipe-language-support-$(node -p "require('./package.json').version").vsix"
  )
}

build_icons() {
  python3 "$REPO_ROOT/packages/pipeon/resolvers/pipeon/assets/scripts/generate-pipeon-icons.py"
}

build_desktop() {
  require_cargo
  mkdir -p "$PIPEON_DESKTOP_TARGET_DIR"
  CARGO_TARGET_DIR="$PIPEON_DESKTOP_TARGET_DIR" \
    cargo build --manifest-path "$REPO_ROOT/packages/pipeon/apps/pipeon-desktop/src-tauri/Cargo.toml" --release
  mkdir -p "$REPO_ROOT/packages/pipeon/apps/pipeon-desktop/bin"
  cp -f "$PIPEON_DESKTOP_TARGET_DIR/release/pipeon-desktop" "$REPO_ROOT/packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop"
  chmod +x "$REPO_ROOT/packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop"
}

install_desktop_global() {
  build_desktop
  mkdir -p "$HOME/.local/share/pipeon/bin"
  mkdir -p "$HOME/.local/share/pipeon/icons"
  mkdir -p "$HOME/.local/share/applications"
  install -m 755 "$REPO_ROOT/packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop" "$HOME/.local/share/pipeon/bin/pipeon-desktop"
  install -m 644 "$REPO_ROOT/packages/pipeon/apps/pipeon-desktop/src-tauri/icons/icon.png" "$HOME/.local/share/pipeon/icons/pipeon.png"
  rm -f "$HOME/.local/share/applications/com.dockpipe.pipeon.desktop"
  printf '%s\n' \
    '[Desktop Entry]' \
    'Type=Application' \
    'Name=Pipeon' \
    "Exec=$HOME/.local/share/pipeon/bin/pipeon-desktop" \
    "Icon=$HOME/.local/share/pipeon/icons/pipeon.png" \
    'Terminal=false' \
    'Categories=Development;' \
    'StartupNotify=true' \
    'StartupWMClass=com.dockpipe.pipeon' \
    > "$HOME/.local/share/applications/com.dockpipe.pipeon.desktop"
}

package_vscode_extension() {
  package_dockpipe_language_support
  mkdir -p "$REPO_ROOT/bin/.dockpipe/extensions"
  rm -rf "$PIPEON_VSCODE_EXT_BUILD_DIR"
  mkdir -p "$PIPEON_VSCODE_EXT_BUILD_DIR"
  cp "$PIPEON_VSCODE_EXT_SRC/package.json" "$PIPEON_VSCODE_EXT_BUILD_DIR/package.json"
  cp "$PIPEON_VSCODE_EXT_SRC/package-lock.json" "$PIPEON_VSCODE_EXT_BUILD_DIR/package-lock.json"
  cp "$PIPEON_VSCODE_EXT_SRC/tsconfig.json" "$PIPEON_VSCODE_EXT_BUILD_DIR/tsconfig.json"
  cp -R "$PIPEON_VSCODE_EXT_SRC/src" "$PIPEON_VSCODE_EXT_BUILD_DIR/src"
  cp -R "$PIPEON_VSCODE_EXT_SRC/types" "$PIPEON_VSCODE_EXT_BUILD_DIR/types"
  cp -R "$PIPEON_VSCODE_EXT_SRC/scripts" "$PIPEON_VSCODE_EXT_BUILD_DIR/scripts"
  NPM_CONFIG_CACHE="$PIPEON_VSCODE_EXT_NPM_CACHE" npm --prefix "$PIPEON_VSCODE_EXT_BUILD_DIR" ci --no-audit --no-fund
  npm --prefix "$PIPEON_VSCODE_EXT_BUILD_DIR" run build
  node "$PIPEON_VSCODE_EXT_BUILD_DIR/scripts/webview-smoke.js"
  install -m 644 "$PIPEON_VSCODE_EXT_BUILD_DIR/extension.js" "$PIPEON_VSCODE_EXT_SRC/extension.js"
  install -m 644 "$PIPEON_VSCODE_EXT_BUILD_DIR/webview/canary.js" "$PIPEON_VSCODE_EXT_SRC/webview/canary.js"
  install -m 644 "$PIPEON_VSCODE_EXT_BUILD_DIR/webview/chat.js" "$PIPEON_VSCODE_EXT_SRC/webview/chat.js"
  (
    cd "$PIPEON_VSCODE_EXT_SRC"
    node ../../../../../src/app/tooling/vscode-extensions/dockpipe-language-support/node_modules/@vscode/vsce/vsce \
      package --no-dependencies \
      -o "$REPO_ROOT/bin/.dockpipe/extensions/pipeon-$(node -p "require('./package.json').version").vsix"
  )
}

install_vscode_extension() {
  package_vscode_extension
  local vsix
  vsix="$(ls -1t "$REPO_ROOT"/bin/.dockpipe/extensions/pipeon-*.vsix | head -n1)"
  local installed=0
  if command -v cursor >/dev/null 2>&1; then
    echo "[dockpipe] installing Pipeon into Cursor: $vsix"
    cursor --install-extension "$vsix" --force
    installed=1
  fi
  if command -v code >/dev/null 2>&1; then
    echo "[dockpipe] installing Pipeon into VS Code: $vsix"
    code --install-extension "$vsix" --force
    installed=1
  fi
  if [[ "$installed" -eq 0 ]]; then
    echo "[dockpipe] no editor CLI found. Install manually from VSIX: $vsix"
  fi
}

build_code_server_image() {
  package_vscode_extension
  docker build -t dockpipe-code-server:latest -f "$PIPEON_VSCODE_EXT_SRC/Dockerfile.code-server" "$REPO_ROOT"
}

main() {
  local cmd="${1:-}"
  case "$cmd" in
    source|"")
      build_source
      ;;
    icons)
      build_icons
      ;;
    desktop)
      build_desktop
      ;;
    install-desktop-global)
      install_desktop_global
      ;;
    dockpipe-language-support)
      package_dockpipe_language_support
      ;;
    vscode-extension)
      package_vscode_extension
      ;;
    install-vscode-extension)
      install_vscode_extension
      ;;
    code-server-image)
      build_code_server_image
      ;;
    -h|--help|help)
      usage
      ;;
    *)
      echo "unknown command: $cmd" >&2
      usage >&2
      exit 1
      ;;
  esac
}

main "$@"
