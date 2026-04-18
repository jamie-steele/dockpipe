#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/repo-tools.sh
source "$SCRIPT_DIR/lib/repo-tools.sh"
WORKFLOW_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"

cd "$ROOT"

BIN="${DORKPIPE_BIN:-}"
if [[ -z "$BIN" ]]; then
  BIN="$(dorkpipe_orchestrator_resolve_dorkpipe_bin "$ROOT")"
fi
SPEC="${DORKPIPE_SPEC:-$WORKFLOW_ROOT/spec.example.yaml}"

if [[ ! -x "$BIN" ]]; then
  echo "dorkpipe: build the orchestrator first: make maintainer-tools (writes packages/dorkpipe/bin/dorkpipe)" >&2
  exit 1
fi

exec "$BIN" run -f "$SPEC" --workdir "$ROOT"
