#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:?DOCKPIPE_SCRIPT_DIR is required}"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../../dorkpipe/assets/scripts/lib/dorkpipe-cli.sh"
ROOT="${DOCKPIPE_WORKDIR:?DOCKPIPE_WORKDIR is required}"
cd "$ROOT"
WORKFLOW_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

BIN="$(dorkpipe_script_resolve_bin "$(dorkpipe_script_repo_root "$SCRIPT_DIR")")"
SPEC="${DORKPIPE_SPEC:-$WORKFLOW_ROOT/spec.example.yaml}"

if [[ ! -x "$BIN" ]]; then
  echo "dorkpipe: build the orchestrator first: ./src/bin/dockpipe package build source --workdir . --only dorkpipe" >&2
  exit 1
fi

exec "$BIN" run -f "$SPEC" --workdir "$ROOT"
