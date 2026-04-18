#!/usr/bin/env bash
# Split insights.json into bin/.dockpipe/analysis/by-category/<category>.json (arrays only; canonical file remains insights.json).
set -euo pipefail

eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"
ROOT="$(dockpipe_sdk workdir)"
OUT="$ROOT/bin/.dockpipe/analysis"
INS="$OUT/insights.json"
CATDIR="$OUT/by-category"

if ! command -v jq >/dev/null 2>&1; then
	echo "user-insight-export-by-category: jq is required" >&2
	exit 1
fi

[[ -f "$INS" ]] || {
	echo "user-insight-export-by-category: missing $INS" >&2
	exit 1
}

mkdir -p "$CATDIR"
for c in risk constraint convention architecture_note compliance future_work unknown; do
	jq --arg c "$c" '[.insights[] | select(.category == $c)]' "$INS" >"$CATDIR/${c}.json"
done

echo "user-insight-export-by-category: wrote $CATDIR/*.json"
