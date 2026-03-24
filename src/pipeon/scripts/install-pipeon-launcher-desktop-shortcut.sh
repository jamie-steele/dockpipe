#!/usr/bin/env bash
# Install Freedesktop menu entry for the Qt Pipeon Launcher (system tray, dockpipe contexts).
# Pop!_OS / GNOME / KDE: appears in Applications after install (may need log out/in for menu cache).
#
# Prerequisite: build the binary first —  make pipeon-launcher
#
# Usage: from repo root —  bash src/pipeon/scripts/install-pipeon-launcher-desktop-shortcut.sh
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
ICON_PNG="${REPO}/src/contrib/pipeon-vscode-extension/images/icon.png"
ICON_SVG="${REPO}/src/templates/core/resolvers/code-server/assets/images/code-server/favicon.svg"
BIN="${REPO}/src/apps/pipeon-launcher/build/pipeon-launcher"

if [[ "$(uname -s 2>/dev/null || echo unknown)" != Linux ]]; then
  echo "This script installs a Linux Freedesktop shortcut only." >&2
  echo "On macOS use: make install-pipeon-shortcut-macos" >&2
  echo "On Windows use: make install-pipeon-shortcut-windows" >&2
  exit 1
fi

if [[ ! -x "$BIN" ]]; then
  echo "Missing executable: $BIN" >&2
  echo "Build it first:  make pipeon-launcher" >&2
  exit 1
fi

if [[ ! -f "$ICON_PNG" ]]; then
  echo "Missing ${ICON_PNG} — run from dockpipe repo root (try: make pipeon-icons)" >&2
  exit 1
fi

ICON_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor"
APP_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"

mkdir -p "$ICON_DIR/128x128/apps" "$ICON_DIR/scalable/apps" "$APP_DIR"
cp "$ICON_PNG" "$ICON_DIR/128x128/apps/pipeon.png"
if [[ -f "$ICON_SVG" ]]; then
  cp "$ICON_SVG" "$ICON_DIR/scalable/apps/pipeon.svg"
fi

gtk-update-icon-cache -f -t "$ICON_DIR" 2>/dev/null || true

DESKTOP="$APP_DIR/pipeon-launcher.desktop"
cat > "$DESKTOP" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Pipeon Launcher
Comment=Tray app: Pipeon contexts and dockpipe runs (Qt)
Exec=${BIN}
Icon=pipeon
Terminal=false
Categories=Development;Utility;
Keywords=dockpipe;pipeon;container;
StartupNotify=true
EOF

chmod 644 "$DESKTOP"
update-desktop-database "$APP_DIR" 2>/dev/null || true

echo "Installed: $DESKTOP"
echo "  Exec: $BIN"
echo "If the icon does not show: gtk-update-icon-cache -f -t ~/.local/share/icons/hicolor"
echo "Search the app menu for \"Pipeon Launcher\"."
