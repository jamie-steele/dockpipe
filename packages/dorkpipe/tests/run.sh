#!/usr/bin/env bash
# Self-contained shell tests for the dorkpipe maintainer package (resolver scripts).
# From repo root: bash packages/dorkpipe/tests/run.sh
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
DIR="$ROOT/packages/dorkpipe/tests"
failed=0
for f in test_normalize_ci_scans.sh test_user_insight_queue.sh test_repo_tools.sh; do
	echo "--- dorkpipe/tests/$f ---"
	bash "$DIR/$f" || failed=1
done
if [[ $failed -ne 0 ]]; then
	exit 1
fi
echo "dorkpipe/tests/run.sh OK"
