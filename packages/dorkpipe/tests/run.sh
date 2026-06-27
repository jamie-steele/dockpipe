#!/usr/bin/env bash
# Self-contained shell tests for the dorkpipe maintainer package (resolver scripts).
# From repo root: dockpipe package test --only dorkpipe
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
DIR="$ROOT/packages/dorkpipe/tests"
failed=0
for f in test_normalize_ci_scans.sh test_user_insight_queue.sh test_repo_tools.sh test_orchestration_lanes.sh test_orchestration_container_auth.sh; do
	echo "--- dorkpipe/tests/$f ---"
	bash "$DIR/$f" || failed=1
done
echo "--- dorkpipe skills.render smoke ---"
DOCKPIPE_ASSETS_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets" \
	DOCKPIPE_ARGS_JSON='["--target","generic","--output","/tmp/dorkpipe-skills-render-test","--dry-run","--skills","dorkpipe-core-review"]' \
	bash "$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts/skills-render.sh" || failed=1
if [[ $failed -ne 0 ]]; then
	exit 1
fi
echo "dorkpipe/tests/run.sh OK"
