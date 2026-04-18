#!/usr/bin/env bash
# Self-contained shell tests for the dorkpipe maintainer package (resolver scripts).
# From repo root: bash packages/dorkpipe/tests/run.sh
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
failed=0
for f in test_normalize_ci_scans.sh test_user_insight_queue.sh test_repo_tools.sh; do
	echo "--- dorkpipe/tests/$f ---"
	bash "$DIR/$f" || failed=1
done
if [[ $failed -ne 0 ]]; then
	exit 1
fi
echo "dorkpipe/tests/run.sh OK"
