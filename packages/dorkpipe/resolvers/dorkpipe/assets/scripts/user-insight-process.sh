#!/usr/bin/env bash
# Normalize new queue items into bin/.dockpipe/analysis/insights.json using deterministic rules (user-insight-rules.json).
set -euo pipefail

eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"
ROOT="$(dockpipe_sdk workdir)"
OUT="$ROOT/bin/.dockpipe/analysis"
SCRIPT_DIR="$(dockpipe_sdk script-dir)"
JQF="$SCRIPT_DIR/jq/process-user-insight-queue.jq"
RULES="$SCRIPT_DIR/user-insight-rules.json"
QUEUE="$OUT/queue.json"
INS="$OUT/insights.json"

if ! command -v jq >/dev/null 2>&1; then
	echo "user-insight-process: jq is required" >&2
	exit 1
fi

mkdir -p "$OUT"
[[ -f "$RULES" ]] || {
	echo "user-insight-process: missing $RULES" >&2
	exit 1
}
[[ -f "$QUEUE" ]] || {
	echo "user-insight-process: no queue at $QUEUE (run user-insight-enqueue.sh first)" >&2
	exit 1
}

if [[ ! -f "$INS" ]]; then
	echo 'null' >"$INS"
fi

NOW="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
tmp="$(mktemp)"
jq -n \
	--arg now "$NOW" \
	--slurpfile q "$QUEUE" \
	--slurpfile i "$INS" \
	--slurpfile r "$RULES" \
	-f "$JQF" >"$tmp"
mv "$tmp" "$INS"

NEWN="$(jq '.provenance.new_insights_count // 0' "$INS")"
echo "user-insight-process: wrote $INS (new normalized insights this run: $NEWN)"

HIST="$OUT/history.jsonl"
jq -n \
	--arg ev "process" \
	--arg ts "$NOW" \
	--argjson n "$NEWN" \
	--argjson prov "$(jq '.provenance' "$INS")" \
	'{event: $ev, at_utc: $ts, new_insights_count: $n, provenance: $prov}' >>"$HIST"
