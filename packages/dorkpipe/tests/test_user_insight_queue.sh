#!/usr/bin/env bash
# Smoke test for the `dorkpipe insight ...` user-insight flow (jq only used for assertions).
# Run from repo root: bash packages/dorkpipe/tests/test_user_insight_queue.sh
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
export PATH="$ROOT/src/bin${PATH:+:$PATH}"

if ! command -v jq >/dev/null 2>&1; then
	echo "test_user_insight_queue: skip (jq not installed)"
	exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

export DOCKPIPE_WORKDIR="$tmp"
export DOCKPIPE_SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
bash "$DOCKPIPE_SCRIPT_DIR/user-insight-enqueue.sh" -m 'convention: use gofmt for Go.' >/dev/null
bash "$DOCKPIPE_SCRIPT_DIR/user-insight-enqueue.sh" -m 'SOC2 review will cover secret storage.' >/dev/null
echo 'null' >"$tmp/bin/.dockpipe/analysis/insights.json"
bash "$DOCKPIPE_SCRIPT_DIR/user-insight-process.sh"

if ! jq -e '
  .kind == "dockpipe_user_insights"
  and (.insights | length == 2)
  and ([.insights[].category] | sort == ["compliance", "convention"])
  and ([.insights[].status] | sort == ["accepted", "pending"])
' "$tmp/bin/.dockpipe/analysis/insights.json" >/dev/null; then
	echo "test_user_insight_queue: insights.json shape unexpected" >&2
	jq '.' "$tmp/bin/.dockpipe/analysis/insights.json" >&2 || true
	exit 1
fi

bash "$DOCKPIPE_SCRIPT_DIR/user-insight-export-by-category.sh"
if ! jq -e 'length >= 1' "$tmp/bin/.dockpipe/analysis/by-category/convention.json" >/dev/null; then
	echo "test_user_insight_queue: by-category export unexpected" >&2
	exit 1
fi

echo "test_user_insight_queue OK"
