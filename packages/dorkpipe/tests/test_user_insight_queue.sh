#!/usr/bin/env bash
# Smoke test for the `dorkpipe insight ...` user-insight flow.
# Run from repo root: bash packages/dorkpipe/tests/test_user_insight_queue.sh
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
export PATH="$ROOT/src/bin${PATH:+:$PATH}"
# shellcheck source=packages/dorkpipe/tests/lib/test-tools.sh
source "$ROOT/packages/dorkpipe/tests/lib/test-tools.sh"
dorkpipe_test_require_go "test_user_insight_queue"
dorkpipe_test_init_go_cache "$ROOT"

tmp="$(dorkpipe_test_mktemp_dir "$ROOT")"
trap 'rm -rf "$tmp"' EXIT

export DOCKPIPE_WORKDIR="$tmp"
export DOCKPIPE_SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
INSIGHTS_PATH="$("$ROOT/src/bin/dockpipe" scope --package dorkpipe analysis/insights.json --workdir "$tmp")"
INSIGHTS_BY_CATEGORY="$("$ROOT/src/bin/dockpipe" scope --package dorkpipe analysis/by-category --workdir "$tmp")"
bash "$DOCKPIPE_SCRIPT_DIR/user-insight-enqueue.sh" -m 'convention: use gofmt for Go.' >/dev/null
bash "$DOCKPIPE_SCRIPT_DIR/user-insight-enqueue.sh" -m 'SOC2 review will cover secret storage.' >/dev/null
mkdir -p "$(dirname "$INSIGHTS_PATH")"
echo 'null' >"$INSIGHTS_PATH"
bash "$DOCKPIPE_SCRIPT_DIR/user-insight-process.sh"

if ! dorkpipe_test_assert "$ROOT" insights-main "$INSIGHTS_PATH"; then
	echo "test_user_insight_queue: insights.json shape unexpected" >&2
	cat "$INSIGHTS_PATH" >&2 || true
	exit 1
fi

bash "$DOCKPIPE_SCRIPT_DIR/user-insight-export-by-category.sh"
if ! dorkpipe_test_assert "$ROOT" insights-category "$INSIGHTS_BY_CATEGORY/convention.json"; then
	echo "test_user_insight_queue: by-category export unexpected" >&2
	exit 1
fi

echo "test_user_insight_queue OK"
