#!/usr/bin/env bash
# Launched by the Pipeon desktop shortcut. Starts workflow vscode (code-server + Pipeon image).
set -euo pipefail
eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"

DOCKPIPE_BIN="$(dockpipe_sdk require dockpipe-bin)" || {
	echo "pipeon-code-server-launch: dockpipe not found; set DOCKPIPE_BIN or add dockpipe to PATH" >&2
	exit 1
}

cd "${PIPEON_WORKDIR:-$HOME}"
exec "$DOCKPIPE_BIN" --workflow vscode --workdir .
