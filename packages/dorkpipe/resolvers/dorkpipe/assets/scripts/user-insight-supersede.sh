#!/usr/bin/env bash
# Link a newer insight to an older one: sets supersedes on NEW and marks OLD superseded + stale.
# Usage: user-insight-supersede.sh <new_insight_id> <old_insight_id>
set -euo pipefail

ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
INS="$ROOT/bin/.dockpipe/analysis/insights.json"
NEW_ID="${1:-}"
OLD_ID="${2:-}"

if [[ -z "$NEW_ID" || -z "$OLD_ID" ]] || [[ ! -f "$INS" ]]; then
	echo "usage: user-insight-supersede.sh <new_insight_id> <old_insight_id>" >&2
	exit 1
fi

NOW="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
tmp="$(mktemp)"
jq --arg new "$NEW_ID" \
	--arg old "$OLD_ID" \
	--arg ts "$NOW" \
	'
  .insights |= map(
    if .id == $new or .queue_item_id == $new then
      . + {supersedes: $old}
      | .history += [{at_utc: $ts, event: "supersede_link", detail: {supersedes: $old}}]
    elif .id == $old or .queue_item_id == $old then
      . + {status: "superseded", stale: true}
      | .history += [{at_utc: $ts, event: "superseded_by", detail: {by: $new}}]
    else . end
  )
' "$INS" >"$tmp"
mv "$tmp" "$INS"

HIST="$ROOT/bin/.dockpipe/analysis/history.jsonl"
jq -n --arg ev "supersede" --arg ts "$NOW" --arg new "$NEW_ID" --arg old "$OLD_ID" \
	'{event: $ev, at_utc: $ts, new_id: $new, old_id: $old}' >>"$HIST"
echo "user-insight-supersede: $NEW_ID supersedes $OLD_ID"
