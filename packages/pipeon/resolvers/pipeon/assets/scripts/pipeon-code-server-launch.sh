#!/usr/bin/env bash
# Launched by the Pipeon desktop shortcut. Starts workflow vscode (code-server + Pipeon image).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/repo-tools.sh
source "$SCRIPT_DIR/lib/repo-tools.sh"

DOCKPIPE_BIN="$(pipeon_resolve_dockpipe_bin)"
[[ -n "$DOCKPIPE_BIN" ]] || {
  echo "pipeon-code-server-launch: dockpipe not found; set DOCKPIPE_BIN or add dockpipe to PATH" >&2
  exit 1
}

cd "${PIPEON_WORKDIR:-$HOME}"
exec "$DOCKPIPE_BIN" --workflow vscode --workdir .
