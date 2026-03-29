#!/usr/bin/env bash
# Smoke test for user insight enqueue + process (requires jq).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if ! command -v jq >/dev/null 2>&1; then
	echo "test_user_insight_queue: skip (jq not installed)"
	exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

export DOCKPIPE_WORKDIR="$tmp"
bash "$ROOT/.staging/packages/dockpipe/agent/dorkpipe/assets/scripts/user-insight-enqueue.sh" -m 'convention: use gofmt for Go.' >/dev/null
bash "$ROOT/.staging/packages/dockpipe/agent/dorkpipe/assets/scripts/user-insight-enqueue.sh" -m 'SOC2 review will cover secret storage.' >/dev/null
echo 'null' >"$tmp/.dockpipe/analysis/insights.json"
bash "$ROOT/.staging/packages/dockpipe/agent/dorkpipe/assets/scripts/user-insight-process.sh"

if ! jq -e '
  .kind == "dockpipe_user_insights"
  and (.insights | length == 2)
  and ([.insights[].category] | sort == ["compliance", "convention"])
  and ([.insights[].status] | sort == ["accepted", "pending"])
' "$tmp/.dockpipe/analysis/insights.json" >/dev/null; then
	echo "test_user_insight_queue: insights.json shape unexpected" >&2
	jq '.' "$tmp/.dockpipe/analysis/insights.json" >&2 || true
	exit 1
fi

bash "$ROOT/.staging/packages/dockpipe/agent/dorkpipe/assets/scripts/user-insight-export-by-category.sh"
if ! jq -e 'length >= 1' "$tmp/.dockpipe/analysis/by-category/convention.json" >/dev/null; then
	echo "test_user_insight_queue: by-category export unexpected" >&2
	exit 1
fi

echo "test_user_insight_queue OK"
