#!/usr/bin/env bash
# Self-contained shell tests for the dorkpipe maintainer package (resolver scripts).
# From repo root: dockpipe package test --only dorkpipe
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
DIR="$ROOT/packages/dorkpipe/tests"
eval "$("$ROOT/src/bin/dockpipe" sdk --workdir "$ROOT")"
export TMPDIR="${DORKPIPE_PACKAGE_TEST_TMPDIR:-$ROOT/bin/.dockpipe/tmp/package-tests}"
mkdir -p "$TMPDIR"
mkdir -p "$(dockpipe_sdk path build go-cache)" "$(dockpipe_sdk path build go-tmp)"
export GOCACHE="${GOCACHE:-$(dockpipe_sdk path build go-cache)}"
export GOTMPDIR="${GOTMPDIR:-$(dockpipe_sdk path build go-tmp)}"
TEST_HOME="${TMPDIR:-/tmp}/dorkpipe-package-test-home-${RANDOM}-${RANDOM}"
mkdir -p "$TEST_HOME"
export HOME="$TEST_HOME"
export USERPROFILE="$TEST_HOME"
export XDG_CONFIG_HOME="$TEST_HOME/.config"
export DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING="${DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING:-never}"
failed=0
for f in test_normalize_ci_scans.sh test_user_insight_queue.sh test_repo_tools.sh test_build_source_operation_results.sh test_orchestration_approval_operation_results.sh test_orchestration_lanes.sh test_orchestration_optimize.sh test_orchestration_container_auth.sh test_dev_stack_gpu_policy.sh; do
	echo "--- dorkpipe/tests/$f ---"
	if ! bash "$DIR/$f"; then
		echo "dorkpipe/tests/$f FAILED" >&2
		failed=1
	fi
done
echo "--- dorkpipe skills.render smoke ---"
if DOCKPIPE_ASSETS_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets" \
	DOCKPIPE_WORKFLOW_NAME="skills.render.smoke" \
	DOCKPIPE_WORKFLOW_CONFIG="$ROOT/packages/dorkpipe/workflows/skills.render/config.yml" \
	DOCKPIPE_STEP_ID="render" \
	DOCKPIPE_ARGS_JSON='["--target","generic","--output","/tmp/dorkpipe-skills-render-test","--dry-run","--skills","dorkpipe-core-review"]' \
	bash "$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts/skills-render.sh"; then
	echo "dorkpipe skills.render smoke OK"
else
	echo "dorkpipe skills.render smoke FAILED" >&2
	failed=1
fi
if [[ $failed -ne 0 ]]; then
	exit 1
fi
echo "dorkpipe/tests/run.sh OK"
