#!/usr/bin/env bash
# Launched by the Pipeon desktop shortcut. Starts workflow vscode (code-server + Pipeon image).
set -euo pipefail

DOCKPIPE_BIN="${DOCKPIPE_BIN:-}"
if [[ -z "$DOCKPIPE_BIN" ]]; then
  DOCKPIPE_BIN="$(command -v dockpipe 2>/dev/null || true)"
fi
[[ -n "$DOCKPIPE_BIN" ]] || {
  echo "pipeon-code-server-launch: dockpipe not found; set DOCKPIPE_BIN or add dockpipe to PATH" >&2
  exit 1
}

cd "${PIPEON_WORKDIR:-$HOME}"
exec "$DOCKPIPE_BIN" --workflow vscode --workdir .
