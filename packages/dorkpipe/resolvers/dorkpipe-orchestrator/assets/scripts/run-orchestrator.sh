#!/usr/bin/env bash
set -euo pipefail

eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"
dockpipe_sdk init-script
WORKFLOW_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

BIN="${DORKPIPE_BIN:-}"
if [[ -z "$BIN" ]]; then
  BIN="$(dockpipe_sdk require dorkpipe-bin)"
fi
SPEC="${DORKPIPE_SPEC:-$WORKFLOW_ROOT/spec.example.yaml}"

if [[ ! -x "$BIN" ]]; then
  echo "dorkpipe: build the orchestrator first: make maintainer-tools (writes packages/dorkpipe/bin/dorkpipe)" >&2
  exit 1
fi

exec "$BIN" run -f "$SPEC" --workdir "$ROOT"
