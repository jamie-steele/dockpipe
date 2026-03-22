#!/usr/bin/env bash
# macOS: double-clickable Pipeon.command in ~/Applications (opens Terminal to run dockpipe).
# Usage: from repo root — bash scripts/install-pipeon-shortcut-macos.sh
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${HOME}/Applications"
mkdir -p "$OUT"
TARGET="$OUT/Pipeon.command"

{
  echo '#!/bin/bash'
  echo "export DOCKPIPE_REPO_ROOT=$(printf %q "$REPO")"
  echo 'cd "${PIPEON_WORKDIR:-$HOME}"'
  echo "exec $(printf %q "$REPO/bin/dockpipe") --workflow vscode --workdir ."
} > "$TARGET"
chmod +x "$TARGET"

echo "Installed: $TARGET"
echo "Finder → Applications → Pipeon (opens Terminal). Custom workspace: PIPEON_WORKDIR=/path ./Pipeon.command"
echo "For a Dock icon with the P, use Finder: Get Info on Pipeon.command → paste icon from icon.png (optional)."
