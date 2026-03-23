#!/usr/bin/env bash
# Launched by the Pipeon desktop shortcut. Starts workflow vscode (code-server + Pipeon image).
set -euo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export DOCKPIPE_REPO_ROOT="${DOCKPIPE_REPO_ROOT:-$REPO}"
cd "${PIPEON_WORKDIR:-$HOME}"
exec "$REPO/bin/dockpipe" --workflow vscode --workdir .
