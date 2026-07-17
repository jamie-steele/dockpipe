#!/usr/bin/env bash
set -euo pipefail
trap 'rc=$?; echo "test_example_brain_contract failed at line ${LINENO}: ${BASH_COMMAND}" >&2; exit "$rc"' ERR

REPO_ROOT="$(git rev-parse --show-toplevel)"
# shellcheck source=packages/dorkpipe/tests/lib/test-tools.sh
source "$REPO_ROOT/packages/dorkpipe/tests/lib/test-tools.sh"
dorkpipe_test_require_go "test_example_brain_contract"
dorkpipe_test_init_go_cache "$REPO_ROOT"

(
  cd "$REPO_ROOT/packages/dorkpipe/lib"
  go test ./orchestrationhelper -run '^TestExampleBrain(BaselineEligibilityInMixedTaskPack|UnchangedConfigurationProvesTaskPackContract)$'
)

echo "test_example_brain_contract OK"
