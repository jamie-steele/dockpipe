#!/usr/bin/env bash
# Bounded, deterministic signals for routing (extend per repo).
set -euo pipefail
ROOT="${DOCKPIPE_WORKDIR:?DOCKPIPE_WORKDIR is required}"
cd "$ROOT"
OUT="${ROOT}/bin/.dockpipe/packages/dorkpipe"
mkdir -p "$OUT"
{
	echo "# task signals $(date -u +%Y-%m-%dT%H:%M:%SZ)"
	test -f go.mod && echo "has_go_mod=1" || echo "has_go_mod=0"
} >"$OUT/task-signals.env"
echo "wrote $OUT/task-signals.env"
