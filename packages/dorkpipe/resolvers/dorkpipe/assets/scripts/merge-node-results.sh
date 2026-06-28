#!/usr/bin/env bash
# Concatenate node outputs under bin/.dockpipe/packages/dorkpipe/nodes/ into one file for downstream prompts.
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
DORKPIPE_STATE_DIR="$(dockpipe_sdk scope --package dorkpipe .)"
NODES="$DORKPIPE_STATE_DIR/nodes"
OUT="$DORKPIPE_STATE_DIR/merged-nodes.txt"
mkdir -p "$(dirname "$OUT")"
if [[ ! -d "$NODES" ]]; then
	echo "(no nodes yet)" >"$OUT"
	exit 0
fi
{
	echo "# merged node outputs"
	find "$NODES" -type f -name '*.txt' | sort | while read -r f; do
		echo ""
		echo "----- $(basename "$f") -----"
		cat "$f"
	done
} >"$OUT"
echo "wrote $OUT"
