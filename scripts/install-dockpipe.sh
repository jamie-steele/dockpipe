#!/usr/bin/env bash
# Install a pre-built dockpipe binary to a local PATH directory. Does not compile.
# Default binary: repo-root bin/dockpipe.bin (override with DOCKPIPE_INSTALL_BIN).
# Override destination root with DOCKPIPE_INSTALL_PREFIX (e.g. /opt/dockpipe — installs to $PREFIX/bin/dockpipe).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${DOCKPIPE_INSTALL_BIN:-$REPO_ROOT/bin/dockpipe.bin}"

if [[ ! -f "$SRC" ]]; then
  echo "install-dockpipe: missing binary: $SRC" >&2
  echo "Run: make build" >&2
  exit 1
fi

if [[ -n "${DOCKPIPE_INSTALL_PREFIX:-}" ]]; then
  DEST_DIR="${DOCKPIPE_INSTALL_PREFIX%/}/bin"
  mkdir -p "$DEST_DIR"
  cp "$SRC" "$DEST_DIR/dockpipe"
  chmod +x "$DEST_DIR/dockpipe"
  echo "Installed: $DEST_DIR/dockpipe"
  exit 0
fi

# Windows (Git Bash / MSYS): put dockpipe.exe next to user home bin
if [[ -n "${WINDIR:-}" ]] || [[ "$(uname -s 2>/dev/null)" == *[Mm][Ss][Yy][Ss]* ]] || [[ "$(uname -s 2>/dev/null)" == *MINGW* ]]; then
  BASE="${USERPROFILE:-$HOME}"
  DEST_DIR="${BASE}/bin"
  mkdir -p "$DEST_DIR"
  cp "$SRC" "$DEST_DIR/dockpipe.exe"
  chmod +x "$DEST_DIR/dockpipe.exe" 2>/dev/null || true
  echo "Installed: $DEST_DIR/dockpipe.exe"
  echo "Add to PATH if needed: $DEST_DIR"
  exit 0
fi

# Unix: prefer ~/.local/bin, then /usr/local/bin (try next if mkdir/cp fails)
if [[ -n "${HOME:-}" ]]; then
  DEST_DIR="$HOME/.local/bin"
  if mkdir -p "$DEST_DIR" 2>/dev/null && cp "$SRC" "$DEST_DIR/dockpipe" 2>/dev/null && chmod +x "$DEST_DIR/dockpipe"; then
    echo "Installed: $DEST_DIR/dockpipe"
    echo "Ensure ~/.local/bin is on your PATH (many distros include it by default)."
    exit 0
  fi
fi

if [[ -d /usr/local/bin ]] && cp "$SRC" "/usr/local/bin/dockpipe" 2>/dev/null && chmod +x "/usr/local/bin/dockpipe"; then
  echo "Installed: /usr/local/bin/dockpipe"
  exit 0
fi

echo "install-dockpipe: no writable install location (tried ~/.local/bin and /usr/local/bin)." >&2
echo "Set DOCKPIPE_INSTALL_PREFIX to a directory; the binary is installed to \$DOCKPIPE_INSTALL_PREFIX/bin/dockpipe" >&2
exit 1
