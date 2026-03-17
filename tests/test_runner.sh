#!/usr/bin/env bash
# Tests for lib/runner.sh: ensure dockpipe_run is defined and run_args are built as expected.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

test_runner_sources() {
  # Source runner and check dockpipe_run exists
  source "${REPO_ROOT}/lib/runner.sh"
  type dockpipe_run | grep -q function
  echo "test_runner_sources OK"
}

test_runner_requires_image() {
  source "${REPO_ROOT}/lib/runner.sh"
  unset DOCKPIPE_IMAGE
  local out
  out=$( ( dockpipe_run true ) 2>&1 ) || true
  if echo "$out" | grep -q "DOCKPIPE_IMAGE"; then
    echo "test_runner_requires_image OK"
  else
    echo "test_runner_requires_image FAIL: should require DOCKPIPE_IMAGE (got: $out)"
    return 1
  fi
}

run_tests() {
  test_runner_sources
  test_runner_requires_image
}

run_tests
