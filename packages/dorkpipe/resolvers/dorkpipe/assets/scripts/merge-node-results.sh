#!/usr/bin/env bash
# Concatenate node outputs under bin/.dockpipe/packages/dorkpipe/nodes/ into one file for downstream prompts.
set -euo pipefail
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
cd "$ROOT"
NODES="${ROOT}/bin/.dockpipe/packages/dorkpipe/nodes"
OUT="${ROOT}/bin/.dockpipe/packages/dorkpipe/merged-nodes.txt"
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
