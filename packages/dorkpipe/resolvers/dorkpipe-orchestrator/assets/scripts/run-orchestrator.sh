#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKFLOW_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"

cd "$ROOT"

resolve_dorkpipe_bin() {
  local configured="${DORKPIPE_BIN:-}"
  local candidate
  if [[ -n "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi
  for candidate in \
    "$ROOT/packages/dorkpipe/bin/dorkpipe" \
    "$WORKFLOW_ROOT/../../../../packages/dorkpipe/bin/dorkpipe"
  do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  command -v dorkpipe 2>/dev/null || true
}

BIN="${DORKPIPE_BIN:-}"
if [[ -z "$BIN" ]]; then
  BIN="$(resolve_dorkpipe_bin)"
fi
SPEC="${DORKPIPE_SPEC:-$WORKFLOW_ROOT/spec.example.yaml}"

if [[ ! -x "$BIN" ]]; then
  echo "dorkpipe: build the orchestrator first: make maintainer-tools (writes packages/dorkpipe/bin/dorkpipe)" >&2
  exit 1
fi

exec "$BIN" run -f "$SPEC" --workdir "$ROOT"
