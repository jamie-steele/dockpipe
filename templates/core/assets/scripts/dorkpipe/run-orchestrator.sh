#!/usr/bin/env bash
# Invoked from dockpipe/workflows/dorkpipe-orchestrator (skip_container host step).
set -euo pipefail
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
cd "$ROOT"
BIN="${DORKPIPE_BIN:-${ROOT}/bin/dorkpipe}"
SPEC="${DORKPIPE_SPEC:-${ROOT}/dockpipe/workflows/dorkpipe-orchestrator/spec.example.yaml}"
if [[ ! -x "$BIN" ]]; then
	echo "dorkpipe: build the orchestrator first: make build (produces bin/dorkpipe)" >&2
	exit 1
fi
exec "$BIN" run -f "$SPEC" --workdir "$ROOT"
