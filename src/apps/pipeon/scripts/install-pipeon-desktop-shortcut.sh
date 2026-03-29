#!/usr/bin/env bash
# Install a Pipeon menu shortcut for code-server in the browser (dockpipe --workflow vscode).
# NOT the Qt tray app — for that see install-pipeon-launcher-desktop-shortcut.sh / make install-pipeon-launcher-shortcut.
# Freedesktop: ~/.local/share/applications/pipeon-code-server.desktop
# Usage: from repo root — bash src/apps/pipeon/scripts/install-pipeon-desktop-shortcut.sh
# Windows: use src/apps/pipeon/scripts/install-pipeon-desktop-shortcut.ps1 or: make install-pipeon-shortcut-windows
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
ICON_PNG="${REPO}/src/contrib/pipeon-vscode-extension/images/icon.png"
ICON_SVG="${REPO}/src/contrib/pipeon-vscode-extension/images/favicon.svg"
LAUNCH="${REPO}/src/apps/pipeon/scripts/pipeon-code-server-launch.sh"

for f in "$ICON_PNG" "$LAUNCH"; do
  if [[ ! -f "$f" ]]; then
    echo "Missing ${f} — run from dockpipe repo root (try: make pipeon-icons)" >&2
    exit 1
  fi
done

chmod +x "$LAUNCH"

ICON_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor"
APP_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"

mkdir -p "$ICON_DIR/128x128/apps" "$ICON_DIR/scalable/apps" "$APP_DIR"
cp "$ICON_PNG" "$ICON_DIR/128x128/apps/pipeon.png"
if [[ -f "$ICON_SVG" ]]; then
  cp "$ICON_SVG" "$ICON_DIR/scalable/apps/pipeon.svg"
fi

gtk-update-icon-cache -f -t "$ICON_DIR" 2>/dev/null || true

DESKTOP="$APP_DIR/pipeon-code-server.desktop"
cat > "$DESKTOP" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Pipeon
Comment=Browser editor (code-server) with Pipeon — opens with your home folder as workspace
Exec=${LAUNCH}
Icon=pipeon
Terminal=false
Categories=Development;IDE;
Keywords=editor;vscode;code-server;pipeon;
StartupNotify=true
EOF

chmod 644 "$DESKTOP"
update-desktop-database "$APP_DIR" 2>/dev/null || true

echo "Installed: $DESKTOP"
echo "  Icon: pipeon (128px PNG + scalable SVG)"
echo "  Opens with workspace = \$HOME (override: PIPEON_WORKDIR=/path/to/project $LAUNCH)"
echo "If the icon does not show, run: gtk-update-icon-cache -f -t ~/.local/share/icons/hicolor"
