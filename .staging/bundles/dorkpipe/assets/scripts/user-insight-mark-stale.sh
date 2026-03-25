#!/usr/bin/env bash
# Mark an insight stale (lifecycle; does not delete provenance).
# Usage: user-insight-mark-stale.sh <insight-or-queue-id>
set -euo pipefail

ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
INS="$ROOT/.dockpipe/analysis/insights.json"
ID="${1:-}"

if [[ -z "$ID" ]] || [[ ! -f "$INS" ]]; then
	echo "usage: user-insight-mark-stale.sh <insight-or-queue-id>" >&2
	exit 1
fi

NOW="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
tmp="$(mktemp)"
jq --arg id "$ID" \
	--arg ts "$NOW" \
	'
  .insights |= map(
    if (.id == $id) or (.queue_item_id == $id) then
      . + {stale: true}
      | .history += [{at_utc: $ts, event: "mark_stale", detail: {}}]
    else . end
  )
' "$INS" >"$tmp"
mv "$tmp" "$INS"

HIST="$ROOT/.dockpipe/analysis/history.jsonl"
jq -n --arg ev "mark_stale" --arg ts "$NOW" --arg id "$ID" '{event: $ev, at_utc: $ts, insight_or_queue_id: $id}' >>"$HIST"
echo "user-insight-mark-stale: $ID"
