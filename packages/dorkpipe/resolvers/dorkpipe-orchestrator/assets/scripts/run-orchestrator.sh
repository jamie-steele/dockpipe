#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKFLOW_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"

cd "$ROOT"

BIN="${DORKPIPE_BIN:-}"
if [[ -z "$BIN" ]]; then
  if command -v dorkpipe >/dev/null 2>&1; then
    BIN="$(command -v dorkpipe)"
  else
    BIN="${SCRIPT_DIR}/../../../../bin/dorkpipe"
  fi
fi
SPEC="${DORKPIPE_SPEC:-$WORKFLOW_ROOT/spec.example.yaml}"

if [[ ! -x "$BIN" ]]; then
  echo "dorkpipe: build the orchestrator first: make maintainer-tools (writes packages/dorkpipe/bin/dorkpipe)" >&2
  exit 1
fi

exec "$BIN" run -f "$SPEC" --workdir "$ROOT"
