#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_ROOT="$(cd "${DOCKPIPE_PACKAGE_ROOT:-$SCRIPT_DIR/../..}" && pwd)"

pipeon_abs_dir() {
  local path="${1:-}"
  [[ -n "$path" ]] || return 1
  (cd "$path" 2>/dev/null && pwd)
}

pipeon_is_repo_root() {
  local root="${1:-}"
  [[ -n "$root" ]] || return 1
  [[ -f "$root/packages/pipeon/package.yml" ]] || return 1
  [[ -f "$root/src/core/assets/scripts/lib/dockpipe-sdk.sh" ]] || return 1
}

pipeon_resolve_repo_root() {
  local candidate
  for candidate in "${DOCKPIPE_REPO_ROOT:-}" "${DOCKPIPE_SDK_ROOT:-}" "${DOCKPIPE_WORKDIR:-}"; do
    candidate="$(pipeon_abs_dir "$candidate" 2>/dev/null || true)"
    if pipeon_is_repo_root "$candidate"; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  candidate="$(git -C "$PACKAGE_ROOT" rev-parse --show-toplevel 2>/dev/null || true)"
  candidate="$(pipeon_abs_dir "$candidate" 2>/dev/null || true)"
  if pipeon_is_repo_root "$candidate"; then
    printf '%s\n' "$candidate"
    return 0
  fi
  candidate="$(pipeon_abs_dir "$PACKAGE_ROOT/../.." 2>/dev/null || true)"
  if pipeon_is_repo_root "$candidate"; then
    printf '%s\n' "$candidate"
    return 0
  fi
  printf '%s\n' "$(pipeon_abs_dir "${DOCKPIPE_WORKDIR:-$PACKAGE_ROOT/../..}")"
}

REPO_ROOT="$(pipeon_resolve_repo_root)"
DOCKPIPE_BIN="${DOCKPIPE_BIN:-$REPO_ROOT/src/bin/dockpipe}"
BUILD_ROOT="$REPO_ROOT/bin/.dockpipe/build"

PIPEON_DESKTOP_TARGET_DIR="${PIPEON_DESKTOP_TARGET_DIR:-$BUILD_ROOT/pipeon-desktop-target}"
PIPEON_EXTENSIONS_DIR="${PIPEON_EXTENSIONS_DIR:-$("$DOCKPIPE_BIN" scope --package pipeon extensions --workdir "$REPO_ROOT")}"
PIPEON_VSCODE_EXT_SRC="$REPO_ROOT/packages/pipeon/resolvers/pipeon/vscode-extension"
PIPEON_VSCODE_EXT_BUILD_DIR="${PIPEON_VSCODE_EXT_BUILD_DIR:-$BUILD_ROOT/pipeon-vscode-extension}"
PIPEON_VSCODE_EXT_NPM_CACHE="${PIPEON_VSCODE_EXT_NPM_CACHE:-$BUILD_ROOT/npm-cache}"
DOCKPIPE_VSCODE_EXT_DIR="$REPO_ROOT/src/app/tooling/vscode-extensions/dockpipe-language-support"
DOCKPIPE_VSCODE_TMP_CACHE="$BUILD_ROOT/npm-cache"
PIPEON_WINDOWS_BUILD_HELPER="$REPO_ROOT/packages/pipeon/assets/scripts/build-desktop-windows.ps1"
PIPEON_WINDOWS_VSIX_HELPER="$REPO_ROOT/packages/pipeon/assets/scripts/package-vsix-windows.ps1"

build_log() {
  printf '[pipeon-build] %s\n' "$*" >&2
}

pipeon_linux_desktop_pkg_config_deps=(
  "glib-2.0 >= 2.70"
  "gio-2.0 >= 2.70"
  "gtk+-3.0"
  "webkit2gtk-4.1"
  "javascriptcoregtk-4.1"
  "libsoup-3.0"
)

pipeon_linux_desktop_deps_available() {
  command -v pkg-config >/dev/null 2>&1 || return 1
  pkg-config --exists "${pipeon_linux_desktop_pkg_config_deps[@]}"
}

pipeon_linux_desktop_deps_message() {
  cat >&2 <<'EOF'
[pipeon-build] Pipeon desktop build requires Linux GUI development packages:
[pipeon-build]   glib-2.0 >= 2.70, gio-2.0 >= 2.70, gtk+-3.0, webkit2gtk-4.1,
[pipeon-build]   javascriptcoregtk-4.1, and libsoup-3.0.
EOF
}

pipeon_should_build_desktop_for_source() {
  case "${PIPEON_BUILD_DESKTOP:-auto}" in
    0|false|FALSE|False|no|NO|No|off|OFF|Off)
      build_log "Skipping Pipeon desktop build because PIPEON_BUILD_DESKTOP=${PIPEON_BUILD_DESKTOP}."
      return 1
      ;;
    1|true|TRUE|True|yes|YES|Yes|on|ON|On)
      return 0
      ;;
  esac

  case "$(uname -s)" in
    Linux)
      if pipeon_linux_desktop_deps_available; then
        return 0
      fi
      build_log "Skipping Pipeon desktop build during source build: Linux GUI development packages are not available."
      pipeon_linux_desktop_deps_message
      build_log "Set PIPEON_BUILD_DESKTOP=1 after installing those packages to require the desktop build."
      return 1
      ;;
    *)
      return 0
      ;;
  esac
}

run_with_progress() {
  local label="$1"
  shift

  local interval="${PIPEON_BUILD_PROGRESS_INTERVAL:-15}"
  local cmd_pid=""
  set +e
  "$@" &
  cmd_pid=$!
  local heartbeat_pid=""
  if [[ "$interval" =~ ^[0-9]+$ ]] && (( interval > 0 )); then
    (
      while kill -0 "$cmd_pid" >/dev/null 2>&1; do
        sleep "$interval" || exit 0
        kill -0 "$cmd_pid" >/dev/null 2>&1 || exit 0
        build_log "$label still running..."
      done
    ) &
    heartbeat_pid=$!
  fi

  local status=0
  wait "$cmd_pid"
  status=$?
  if [[ -n "$heartbeat_pid" ]]; then
    kill "$heartbeat_pid" >/dev/null 2>&1 || true
    wait "$heartbeat_pid" >/dev/null 2>&1 || true
  fi
  set -e

  return "$status"
}

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

refresh_cargo_path() {
  if command -v cargo >/dev/null 2>&1; then
    return 0
  fi
  local cargo_bin=""
  case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*)
      if [[ -n "${HOME:-}" && -x "$HOME/.cargo/bin/cargo.exe" ]]; then
        cargo_bin="$HOME/.cargo/bin"
      elif [[ -n "${USERPROFILE:-}" ]]; then
        local userprofile_unix=""
        if command -v cygpath >/dev/null 2>&1; then
          userprofile_unix="$(cygpath -u "$USERPROFILE" 2>/dev/null || true)"
        fi
        if [[ -n "$userprofile_unix" && -x "$userprofile_unix/.cargo/bin/cargo.exe" ]]; then
          cargo_bin="$userprofile_unix/.cargo/bin"
        elif [[ -x "${USERPROFILE//\\//}/.cargo/bin/cargo.exe" ]]; then
          cargo_bin="${USERPROFILE//\\//}/.cargo/bin"
        fi
      fi
      ;;
    *)
      if [[ -n "${HOME:-}" && -x "$HOME/.cargo/bin/cargo" ]]; then
        cargo_bin="$HOME/.cargo/bin"
      fi
      ;;
  esac
  if [[ -n "$cargo_bin" ]]; then
    export PATH="$cargo_bin:$PATH"
  fi
  command -v cargo >/dev/null 2>&1
}

pipeon_powershell_bin() {
  if command -v pwsh.exe >/dev/null 2>&1; then
    printf 'pwsh.exe\n'
    return 0
  fi
  if command -v pwsh >/dev/null 2>&1; then
    printf 'pwsh\n'
    return 0
  fi
  if command -v powershell.exe >/dev/null 2>&1; then
    printf 'powershell.exe\n'
    return 0
  fi
  if command -v powershell >/dev/null 2>&1; then
    printf 'powershell\n'
    return 0
  fi
  echo "[dockpipe] PowerShell was not found on PATH." >&2
  return 1
}

pipeon_is_windows_host() {
  case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*)
      return 0
      ;;
  esac
  return 1
}

pipeon_windows_path() {
  local path_value="$1"
  if command -v cygpath >/dev/null 2>&1; then
    cygpath -w "$path_value"
  else
    printf '%s\n' "$path_value"
  fi
}

package_vsix_windows() {
  local extension_dir="$1"
  local output_file="$2"
  local npm_cache="${3:-}"
  local vsce_entrypoint="${4:-}"
  local powershell_bin
  powershell_bin="$(pipeon_powershell_bin)" || return 1

  local args=(
    -NoProfile
    -ExecutionPolicy Bypass
    -File "$(pipeon_windows_path "$PIPEON_WINDOWS_VSIX_HELPER")"
    -ExtensionDir "$(pipeon_windows_path "$extension_dir")"
    -OutputFile "$(pipeon_windows_path "$output_file")"
  )
  if [[ -n "$npm_cache" ]]; then
    args+=(-NpmCache "$(pipeon_windows_path "$npm_cache")")
  fi
  if [[ -n "$vsce_entrypoint" ]]; then
    args+=(-VsceEntrypoint "$(pipeon_windows_path "$vsce_entrypoint")")
  fi
  "$powershell_bin" "${args[@]}"
}

cargo_is_installed_but_unbound() {
  case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*)
      if [[ -n "${HOME:-}" && -x "$HOME/.cargo/bin/cargo.exe" ]]; then
        return 0
      fi
      if [[ -n "${USERPROFILE:-}" ]]; then
        local userprofile_unix=""
        if command -v cygpath >/dev/null 2>&1; then
          userprofile_unix="$(cygpath -u "$USERPROFILE" 2>/dev/null || true)"
        fi
        if [[ -n "$userprofile_unix" && -x "$userprofile_unix/.cargo/bin/cargo.exe" ]]; then
          return 0
        fi
        if [[ -x "${USERPROFILE//\\//}/.cargo/bin/cargo.exe" ]]; then
          return 0
        fi
      fi
      ;;
    *)
      if [[ -n "${HOME:-}" && -x "$HOME/.cargo/bin/cargo" ]]; then
        return 0
      fi
      ;;
  esac
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
  if refresh_cargo_path; then
    return 0
  fi
  if cargo_is_installed_but_unbound; then
    echo "[dockpipe] Rust appears installed, but Cargo is not visible in the current shell. Open a new terminal and rerun." >&2
    exit 1
  fi
  local answer
  answer="$(prompt_install_cargo)" || answer="no"
  if [[ "$answer" == "yes" ]]; then
    if install_cargo_host; then
      if refresh_cargo_path; then
        echo "[dockpipe] Cargo is now available in the current shell. Continuing Pipeon source build..."
        return 0
      fi
      echo "[dockpipe] Cargo install command finished, but the current shell still cannot see Cargo. Open a new shell if needed, then rerun the Pipeon source build." >&2
      exit 1
    fi
    echo "[dockpipe] Cargo is still unavailable. Finish the Rust install, open a new shell, and rerun." >&2
    exit 1
  fi
  echo "[dockpipe] Pipeon source build requires Cargo. Install Rust/Cargo and rerun." >&2
  exit 1
}

build_source() {
  if pipeon_should_build_desktop_for_source; then
    build_desktop
  fi
  package_vscode_extension
}

package_dockpipe_language_support() {
  build_log "Packaging DockPipe language support VSIX"
  mkdir -p "$PIPEON_EXTENSIONS_DIR"
  (
    local version output_file
    cd "$DOCKPIPE_VSCODE_EXT_DIR"
    if [[ ! -x node_modules/.bin/vsce ]]; then
      build_log "Installing DockPipe language support npm dependencies"
      run_with_progress "DockPipe language support npm install" \
        env NPM_CONFIG_CACHE="$DOCKPIPE_VSCODE_TMP_CACHE" npm ci --no-audit --no-fund
    fi
    version="$(node -p "require('./package.json').version")"
    output_file="$PIPEON_EXTENSIONS_DIR/dockpipe-language-support-$version.vsix"
    build_log "Running vsce package for DockPipe language support"
    if pipeon_is_windows_host; then
      package_vsix_windows "$DOCKPIPE_VSCODE_EXT_DIR" "$output_file" "$DOCKPIPE_VSCODE_TMP_CACHE"
    else
      env NPM_CONFIG_CACHE="$DOCKPIPE_VSCODE_TMP_CACHE" \
        node node_modules/@vscode/vsce/vsce package --no-dependencies \
        -o "$output_file"
    fi
    build_log "DockPipe language support VSIX packaging returned"
  )
}

build_icons() {
  python3 "$REPO_ROOT/packages/pipeon/resolvers/pipeon/assets/scripts/generate-pipeon-icons.py"
}

build_desktop_windows() {
  build_log "Building Pipeon desktop shell for Windows"
  require_cargo
  local powershell_bin
  powershell_bin="$(pipeon_powershell_bin)" || return 1
  run_with_progress "Pipeon desktop Windows build" \
    "$powershell_bin" -NoProfile -ExecutionPolicy Bypass -File "$(pipeon_windows_path "$PIPEON_WINDOWS_BUILD_HELPER")" \
      -RepoRoot "$(pipeon_windows_path "$REPO_ROOT")" \
      -TargetDir "$(pipeon_windows_path "$PIPEON_DESKTOP_TARGET_DIR")"
}

build_desktop() {
  case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*)
      build_desktop_windows
      return 0
      ;;
    Linux)
      if ! pipeon_linux_desktop_deps_available; then
        pipeon_linux_desktop_deps_message
        echo "[pipeon-build] Install the missing packages or run the default source build with PIPEON_BUILD_DESKTOP=0." >&2
        return 1
      fi
      ;;
  esac
  require_cargo
  mkdir -p "$PIPEON_DESKTOP_TARGET_DIR"
  run_with_progress "Pipeon desktop Cargo build" \
    env CARGO_TARGET_DIR="$PIPEON_DESKTOP_TARGET_DIR" \
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
  build_log "Preparing Pipeon VS Code extension build workspace"
  mkdir -p "$PIPEON_EXTENSIONS_DIR"
  rm -rf "$PIPEON_VSCODE_EXT_BUILD_DIR"
  mkdir -p "$PIPEON_VSCODE_EXT_BUILD_DIR"
  cp "$PIPEON_VSCODE_EXT_SRC/package.json" "$PIPEON_VSCODE_EXT_BUILD_DIR/package.json"
  cp "$PIPEON_VSCODE_EXT_SRC/package-lock.json" "$PIPEON_VSCODE_EXT_BUILD_DIR/package-lock.json"
  cp "$PIPEON_VSCODE_EXT_SRC/tsconfig.json" "$PIPEON_VSCODE_EXT_BUILD_DIR/tsconfig.json"
  cp -R "$PIPEON_VSCODE_EXT_SRC/src" "$PIPEON_VSCODE_EXT_BUILD_DIR/src"
  cp -R "$PIPEON_VSCODE_EXT_SRC/types" "$PIPEON_VSCODE_EXT_BUILD_DIR/types"
  cp -R "$PIPEON_VSCODE_EXT_SRC/scripts" "$PIPEON_VSCODE_EXT_BUILD_DIR/scripts"
  build_log "Installing Pipeon VS Code extension npm dependencies"
  run_with_progress "Pipeon VS Code extension npm install" \
    env NPM_CONFIG_CACHE="$PIPEON_VSCODE_EXT_NPM_CACHE" npm --prefix "$PIPEON_VSCODE_EXT_BUILD_DIR" ci --no-audit --no-fund --loglevel=notice
  build_log "Compiling Pipeon VS Code extension"
  run_with_progress "Pipeon VS Code extension compile" \
    npm --prefix "$PIPEON_VSCODE_EXT_BUILD_DIR" run build
  build_log "Running Pipeon webview smoke test"
  run_with_progress "Pipeon webview smoke test" \
    node "$PIPEON_VSCODE_EXT_BUILD_DIR/scripts/webview-smoke.js"
  build_log "Copying built Pipeon extension assets back into source tree"
  install -m 644 "$PIPEON_VSCODE_EXT_BUILD_DIR/extension.js" "$PIPEON_VSCODE_EXT_SRC/extension.js"
  install -m 644 "$PIPEON_VSCODE_EXT_BUILD_DIR/webview/canary.js" "$PIPEON_VSCODE_EXT_SRC/webview/canary.js"
  install -m 644 "$PIPEON_VSCODE_EXT_BUILD_DIR/webview/chat.js" "$PIPEON_VSCODE_EXT_SRC/webview/chat.js"
  (
    local version output_file vsce_entrypoint
    cd "$PIPEON_VSCODE_EXT_SRC"
    build_log "Packaging Pipeon VSIX"
    version="$(node -p "require('./package.json').version")"
    output_file="$PIPEON_EXTENSIONS_DIR/pipeon-$version.vsix"
    vsce_entrypoint="$REPO_ROOT/src/app/tooling/vscode-extensions/dockpipe-language-support/node_modules/@vscode/vsce/vsce"
    if pipeon_is_windows_host; then
      package_vsix_windows "$PIPEON_VSCODE_EXT_SRC" "$output_file" "" "$vsce_entrypoint"
    else
      node "$vsce_entrypoint" \
        package --no-dependencies \
        -o "$output_file"
    fi
    build_log "Pipeon VSIX packaging returned"
  )
}

install_vscode_extension() {
  package_vscode_extension
  local vsix
  vsix="$(ls -1t "$PIPEON_EXTENSIONS_DIR"/pipeon-*.vsix | head -n1)"
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
  build_log "Building docker image dockpipe-code-server:latest"
  run_with_progress "dockpipe-code-server docker build" \
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
