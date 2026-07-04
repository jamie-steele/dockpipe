#!/usr/bin/env bash
# Install a pre-built dockpipe binary to a local PATH directory. Does not compile.
# Default binary: repo-root src/bin/dockpipe.bin (override with DOCKPIPE_INSTALL_BIN).
# Override destination root with DOCKPIPE_INSTALL_PREFIX (e.g. /opt/dockpipe — installs to $PREFIX/bin/dockpipe).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SRC="${DOCKPIPE_INSTALL_BIN:-$REPO_ROOT/src/bin/dockpipe.bin}"

install_binary_atomically() {
  local src="$1"
  local dest="$2"
  local dest_dir
  dest_dir="$(dirname "$dest")"
  mkdir -p "$dest_dir"
  local tmp
  tmp="$(mktemp "$dest_dir/.dockpipe-install.XXXXXX")"
  cp "$src" "$tmp"
  chmod +x "$tmp"
  mv -f "$tmp" "$dest"
}

bundled_cache_base() {
  if [[ -n "${DOCKPIPE_BUNDLED_CACHE:-}" ]]; then
    printf '%s\n' "${DOCKPIPE_BUNDLED_CACHE%/}"
    return 0
  fi
  case "$(uname -s 2>/dev/null || true)" in
    Darwin*)
      printf '%s\n' "${HOME}/Library/Caches"
      ;;
    *[Mm][Ss][Yy][Ss]*|*MINGW*|*CYGWIN*)
      printf '%s\n' "${LOCALAPPDATA:-${USERPROFILE:-$HOME}/AppData/Local}"
      ;;
    *)
      printf '%s\n' "${XDG_CACHE_HOME:-${HOME}/.cache}"
      ;;
  esac
}

invalidate_materialized_bundle_cache() {
  local version cache_base target expected
  version="$(tr -d ' \t\r\n' < "$REPO_ROOT/VERSION")"
  [[ -n "$version" ]] || return 0
  cache_base="$(bundled_cache_base)"
  target="${cache_base%/}/dockpipe/bundled-${version}"
  [[ -d "$target" ]] || return 0
  expected="$(cd "$(dirname "$target")" && pwd -P)/$(basename "$target")"
  target="$(cd "$target" && pwd -P)"
  if [[ "$target" != "$expected" ]]; then
    echo "install-dockpipe: refusing to remove unexpected bundled cache path: $target" >&2
    return 1
  fi
  echo "Invalidating materialized bundled workflow cache at $target"
  rm -rf "$target"
}

if [[ ! -f "$SRC" ]]; then
  echo "install-dockpipe: missing binary: $SRC" >&2
  echo "Run: make build" >&2
  exit 1
fi

if [[ -n "${DOCKPIPE_INSTALL_PREFIX:-}" ]]; then
  DEST_DIR="${DOCKPIPE_INSTALL_PREFIX%/}/bin"
  install_binary_atomically "$SRC" "$DEST_DIR/dockpipe"
  invalidate_materialized_bundle_cache
  echo "Installed: $DEST_DIR/dockpipe"
  exit 0
fi

# Windows (Git Bash / MSYS): put dockpipe.exe next to user home bin
if [[ -n "${WINDIR:-}" ]] || [[ "$(uname -s 2>/dev/null)" == *[Mm][Ss][Yy][Ss]* ]] || [[ "$(uname -s 2>/dev/null)" == *MINGW* ]]; then
  BASE="${USERPROFILE:-$HOME}"
  DEST_DIR="${BASE}/bin"
  install_binary_atomically "$SRC" "$DEST_DIR/dockpipe.exe"
  chmod +x "$DEST_DIR/dockpipe.exe" 2>/dev/null || true
  invalidate_materialized_bundle_cache
  echo "Installed: $DEST_DIR/dockpipe.exe"
  echo "Add to PATH if needed: $DEST_DIR"
  exit 0
fi

# Unix: prefer ~/.local/bin, then /usr/local/bin (try next if install fails).
local_install_err=""
if [[ -n "${HOME:-}" ]]; then
  DEST_DIR="$HOME/.local/bin"
  if install_binary_atomically "$SRC" "$DEST_DIR/dockpipe" 2>/tmp/dockpipe-install-local.err; then
    invalidate_materialized_bundle_cache
    echo "Installed: $DEST_DIR/dockpipe"
    echo "Ensure ~/.local/bin is on your PATH (many distros include it by default)."
    exit 0
  else
    local_install_err="$(cat /tmp/dockpipe-install-local.err 2>/dev/null || true)"
  fi
fi

global_install_err=""
if [[ -d /usr/local/bin ]] && install_binary_atomically "$SRC" "/usr/local/bin/dockpipe" 2>/tmp/dockpipe-install-global.err; then
  invalidate_materialized_bundle_cache
  echo "Installed: /usr/local/bin/dockpipe"
  exit 0
else
  global_install_err="$(cat /tmp/dockpipe-install-global.err 2>/dev/null || true)"
fi

echo "install-dockpipe: no writable install location (tried ~/.local/bin and /usr/local/bin)." >&2
if [[ -n "$local_install_err" ]]; then
  echo "install-dockpipe: ~/.local/bin error: $local_install_err" >&2
fi
if [[ -n "$global_install_err" ]]; then
  echo "install-dockpipe: /usr/local/bin error: $global_install_err" >&2
fi
echo "Set DOCKPIPE_INSTALL_PREFIX to a directory; the binary is installed to \$DOCKPIPE_INSTALL_PREFIX/bin/dockpipe" >&2
exit 1
