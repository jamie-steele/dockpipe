#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../../dorkpipe/assets/scripts/lib/dorkpipe-cli.sh"
dorkpipe_script_bootstrap "$SCRIPT_DIR"
dockpipe_sdk init-script
WORKFLOW_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

BIN="$(dorkpipe_script_resolve_bin "$(dorkpipe_script_repo_root "$SCRIPT_DIR")")"
SPEC="${DORKPIPE_SPEC:-$WORKFLOW_ROOT/spec.example.yaml}"

if [[ ! -x "$BIN" ]]; then
  echo "dorkpipe: build the orchestrator first: make maintainer-tools (writes packages/dorkpipe/bin/dorkpipe)" >&2
  exit 1
fi

exec "$BIN" run -f "$SPEC" --workdir "$ROOT"
