#!/usr/bin/env bash
# Bounded, deterministic signals for routing (extend per repo).
set -euo pipefail
ROOT="${DOCKPIPE_WORKDIR:?DOCKPIPE_WORKDIR is required}"
if [[ -n "${DOCKPIPE_SDK_SH:-}" && -f "$DOCKPIPE_SDK_SH" ]]; then
	# shellcheck source=/dev/null
	source "$DOCKPIPE_SDK_SH"
	dockpipe_sdk_refresh "$ROOT"
else
	eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk --workdir "$ROOT")"
fi
cd "$ROOT"
OUT="$(dockpipe_sdk path package dorkpipe)"
mkdir -p "$OUT"
{
	echo "# task signals $(date -u +%Y-%m-%dT%H:%M:%SZ)"
	test -f go.mod && echo "has_go_mod=1" || echo "has_go_mod=0"
} >"$OUT/task-signals.env"
echo "wrote $OUT/task-signals.env"
