#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Host steps receive argv in DOCKPIPE_ARGS_JSON. Direct fixture tests may pass
# normal argv; both paths enter the exact same package-owned Node contract.
if [[ "$#" -eq 0 && -n "${DOCKPIPE_ARGS_JSON:-}" ]]; then
  exec node "$SCRIPT_DIR/devcontainer-lifecycle.js" --args-json "$DOCKPIPE_ARGS_JSON"
fi

exec node "$SCRIPT_DIR/devcontainer-lifecycle.js" "$@"
