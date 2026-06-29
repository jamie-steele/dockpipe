#!/usr/bin/env bash
# Pipeon helper resolution should prefer the repo-local dockpipe binary.
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"

# shellcheck source=/dev/null
source "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh"

actual="$(DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_sdk require dockpipe-bin' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
expected="$(DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_resolve_dockpipe_bin "$2"' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh" "$ROOT")"

if [[ "$actual" != "$expected" ]]; then
  echo "test_repo_tools: expected $expected, got $actual" >&2
  exit 1
fi

build_cache="$(DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_sdk path build npm-cache' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
if [[ "$build_cache" != "$ROOT/bin/.dockpipe/build/npm-cache" ]]; then
  echo "test_repo_tools: expected build cache under root bin/.dockpipe, got $build_cache" >&2
  exit 1
fi

package_state="$(DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_sdk scope --package dorkpipe dev-stack' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
expected_package_state="$("$ROOT/src/bin/dockpipe" scope --package dorkpipe dev-stack --workdir "$ROOT")"
if [[ "$package_state" != "$expected_package_state" ]]; then
  echo "test_repo_tools: expected package state $expected_package_state, got $package_state" >&2
  exit 1
fi

workflow_state="$(DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_sdk scope workflow docs.orchestrate orchestrate' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
expected_workflow_state="$("$ROOT/src/bin/dockpipe" scope workflow docs.orchestrate orchestrate --workdir "$ROOT")"
if [[ "$workflow_state" != "$expected_workflow_state" ]]; then
  echo "test_repo_tools: expected workflow state $expected_workflow_state, got $workflow_state" >&2
  exit 1
fi

ci_default="$(env -u DOCKPIPE_WORKFLOW_NAME -u DOCKPIPE_CI_RAW_DIR -u DOCKPIPE_CI_ANALYSIS_DIR -u DOCKPIPE_ARTIFACT_ROOT -u DOCKPIPE_OUTPUT_ROOT DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_sdk ci analysis findings.json' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
state_root="$ROOT/bin/.dockpipe"
expected_ci_default="$state_root/packages/dorkpipe/ci/analysis/findings.json"
if [[ "$ci_default" != "$expected_ci_default" ]]; then
  echo "test_repo_tools: expected default CI artifacts under DorkPipe package state, got $ci_default" >&2
  exit 1
fi

ci_default_injected="$(env -u DOCKPIPE_WORKFLOW_NAME -u DOCKPIPE_CI_RAW_DIR -u DOCKPIPE_CI_ANALYSIS_DIR -u DOCKPIPE_ARTIFACT_ROOT -u DOCKPIPE_OUTPUT_ROOT DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; printf "%s\n" "$DOCKPIPE_CI_ANALYSIS_DIR/findings.json"' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
if [[ "$ci_default_injected" != "$expected_ci_default" ]]; then
  echo "test_repo_tools: expected SDK refresh to inject default CI artifacts, got $ci_default_injected" >&2
  exit 1
fi

ci_bound="$(env -u DOCKPIPE_CI_RAW_DIR -u DOCKPIPE_CI_ANALYSIS_DIR -u DOCKPIPE_ARTIFACT_ROOT -u DOCKPIPE_OUTPUT_ROOT DOCKPIPE_WORKDIR="$ROOT" DOCKPIPE_WORKFLOW_NAME=ci bash -lc 'source "$1"; printf "%s\n" "$(dockpipe_sdk ci raw)" "$(dockpipe_sdk ci analysis)"' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
expected_ci_bound="$state_root/workflows/ci/artifacts/ci-raw
$state_root/workflows/ci/artifacts/ci-analysis"
if [[ "$ci_bound" != "$expected_ci_bound" ]]; then
  echo "test_repo_tools: expected workflow-bound CI artifacts, got $ci_bound" >&2
  exit 1
fi

ci_injected="$(env -u DOCKPIPE_CI_RAW_DIR -u DOCKPIPE_CI_ANALYSIS_DIR -u DOCKPIPE_ARTIFACT_ROOT -u DOCKPIPE_OUTPUT_ROOT DOCKPIPE_WORKDIR="$ROOT" DOCKPIPE_WORKFLOW_NAME=ci bash -lc 'source "$1"; printf "%s\n" "$DOCKPIPE_CI_RAW_DIR" "$DOCKPIPE_CI_ANALYSIS_DIR"' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
if [[ "$ci_injected" != "$expected_ci_bound" ]]; then
  echo "test_repo_tools: expected SDK refresh to inject workflow-bound CI dirs, got $ci_injected" >&2
  exit 1
fi

echo "test_repo_tools OK"
