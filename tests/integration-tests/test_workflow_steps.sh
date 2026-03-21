#!/usr/bin/env bash
# Multi-step workflow: outputs.env feeds the next container via DOCKPIPE_EXTRA_ENV.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLI="${REPO_ROOT}/bin/dockpipe"

test_chain_outputs() {
  local tmp
  tmp="$(mktemp -d)"
  (
    cd "$tmp"
    DOCKPIPE_REPO_ROOT="$REPO_ROOT" "$CLI" --workflow test 2>&1 | tee "$tmp/out.log"
    if ! grep -q "step2:hello" "$tmp/out.log"; then
      echo "test_chain_outputs FAIL: expected step2:hello in output"
      cat "$tmp/out.log"
      return 1
    fi
  )
  rm -rf "$tmp"
  echo "test_chain_outputs OK"
}

run_tests() {
  test_chain_outputs
}

run_tests
