#!/usr/bin/env bash
# Append one user insight to bin/.dockpipe/analysis/queue.json (structured capture; not implicit memory).
# Usage: user-insight-enqueue.sh -m 'text' [--category-hint unknown] [--repo-path p] [--component c] [--workflow w]
#        echo 'text' | user-insight-enqueue.sh
set -euo pipefail

eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"
ROOT="$(dockpipe_sdk workdir)"
OUT="$ROOT/bin/.dockpipe/analysis"
mkdir -p "$OUT"
QUEUE="$OUT/queue.json"

RAW=""
CATEGORY_HINT="unknown"
REPO_PATH=""
COMPONENT=""
WORKFLOW=""

while [[ $# -gt 0 ]]; do
	case "$1" in
	-m)
		shift
		RAW="${1:-}"
		shift
		;;
	--category-hint)
		shift
		CATEGORY_HINT="${1:-}"
		shift
		;;
	--repo-path)
		shift
		REPO_PATH="${1:-}"
		shift
		;;
	--component)
		shift
		COMPONENT="${1:-}"
		shift
		;;
	--workflow)
		shift
		WORKFLOW="${1:-}"
		shift
		;;
	*)
		echo "user-insight-enqueue: unknown arg: $1" >&2
		exit 1
		;;
	esac
done

if [[ -z "$RAW" ]]; then
	RAW="$(cat || true)"
fi
if [[ -z "${RAW// }" ]]; then
	echo "usage: user-insight-enqueue.sh -m 'insight text' OR pipe text on stdin" >&2
	exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
	echo "user-insight-enqueue: jq is required" >&2
	exit 1
fi

TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
TS_COMPACT="$(date -u +%Y%m%dT%H%M%SZ)"
NONCE="$(openssl rand -hex 4 2>/dev/null || printf '%08x' "$RANDOM$RANDOM")"
HASH="$(printf '%s|%s|%s' "$RAW" "$TS" "$NONCE" | sha256sum 2>/dev/null | awk '{print $1}' | cut -c1-14)"
[[ -n "$HASH" ]] || HASH="$(printf '%s|%s|%s' "$RAW" "$TS" "$NONCE" | shasum -a 256 2>/dev/null | awk '{print $1}' | cut -c1-14)"
ID="ui-${TS_COMPACT}-${HASH}"

if [[ ! -f "$QUEUE" ]]; then
	jq -n --arg sv "1.0" '{schema_version: $sv, kind: "dockpipe_user_insight_queue", items: []}' >"$QUEUE"
fi

ITEM="$(jq -n \
	--arg id "$ID" \
	--arg raw "$RAW" \
	--arg ch "$CATEGORY_HINT" \
	--arg ts "$TS" \
	--arg rp "$REPO_PATH" \
	--arg comp "$COMPONENT" \
	--arg wf "$WORKFLOW" \
	'{
    id: $id,
    raw_text: $raw,
    category_hint: $ch,
    source: "user",
    timestamp_utc: $ts,
    scope: (
      {
        repo_path: (if $rp != "" then $rp else null end),
        component: (if $comp != "" then $comp else null end),
        workflow: (if $wf != "" then $wf else null end)
      }
      | with_entries(select(.value != null))
      | if length == 0 then null else . end
    )
  }')"

tmp="$(mktemp)"
jq --argjson item "$ITEM" '.items += [$item]' "$QUEUE" >"$tmp"
mv "$tmp" "$QUEUE"

HIST="$OUT/history.jsonl"
jq -n \
	--arg ev "enqueue" \
	--arg id "$ID" \
	--arg ts "$TS" \
	--argjson item "$ITEM" \
	'{event: $ev, at_utc: $ts, queue_item_id: $id, payload: $item}' >>"$HIST"

echo "$ID"
