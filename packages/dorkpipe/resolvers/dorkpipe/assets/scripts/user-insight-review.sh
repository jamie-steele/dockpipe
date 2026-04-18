#!/usr/bin/env bash
# Accept or reject a normalized insight by id (insight-* or queue ui-*).
# Usage: user-insight-review.sh accept|reject <id> [--reason "…"]
set -euo pipefail

eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"
ROOT="$(dockpipe_sdk workdir)"
OUT="$ROOT/bin/.dockpipe/analysis"
INS="$OUT/insights.json"

ACTION="${1:-}"
ID="${2:-}"
REASON=""

if [[ "$ACTION" != "accept" && "$ACTION" != "reject" ]]; then
	echo "usage: user-insight-review.sh accept|reject <insight-or-queue-id> [--reason ...]" >&2
	exit 1
fi
shift 2 || true

while [[ $# -gt 0 ]]; do
	case "$1" in
	--reason)
		shift
		REASON="${1:-}"
		shift
		;;
	*)
		echo "unknown arg: $1" >&2
		exit 1
		;;
	esac
done

if [[ -z "$ID" ]] || [[ ! -f "$INS" ]]; then
	echo "user-insight-review: missing insights file or id" >&2
	exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
	echo "user-insight-review: jq is required" >&2
	exit 1
fi

TARGET_STATUS="accepted"
[[ "$ACTION" == "reject" ]] && TARGET_STATUS="rejected"

NOW="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
tmp="$(mktemp)"
jq --arg id "$ID" \
	--arg st "$TARGET_STATUS" \
	--arg ts "$NOW" \
	--arg reason "$REASON" \
	'
  .insights |= map(
    if (.id == $id) or (.queue_item_id == $id) or ("insight-" + $id == .id) or ("insight-" + .queue_item_id == "insight-" + $id) then
      . + {status: $st}
      | if $st == "rejected" and ($reason != "") then . + {rejection_reason: $reason} else . end
      | .history += [{at_utc: $ts, event: ("review_" + $st), detail: {reason: $reason}}]
    else . end
  )
' "$INS" >"$tmp"
mv "$tmp" "$INS"

HIST="$OUT/history.jsonl"
jq -n \
	--arg ev "review_${ACTION}" \
	--arg ts "$NOW" \
	--arg id "$ID" \
	--arg st "$TARGET_STATUS" \
	--arg reason "$REASON" \
	'{event: $ev, at_utc: $ts, insight_or_queue_id: $id, status: $st, reason: $reason}' >>"$HIST"

echo "user-insight-review: $ACTION $ID -> $TARGET_STATUS"
