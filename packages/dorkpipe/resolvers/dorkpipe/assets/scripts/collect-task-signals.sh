#!/usr/bin/env bash
# Bounded, deterministic signals for routing (extend per repo).
set -euo pipefail
eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"
ROOT="$(dockpipe_sdk workdir)"
dockpipe_sdk cd-workdir
OUT="${ROOT}/bin/.dockpipe/packages/dorkpipe"
mkdir -p "$OUT"
{
	echo "# task signals $(date -u +%Y-%m-%dT%H:%M:%SZ)"
	test -f go.mod && echo "has_go_mod=1" || echo "has_go_mod=0"
} >"$OUT/task-signals.env"
echo "wrote $OUT/task-signals.env"
