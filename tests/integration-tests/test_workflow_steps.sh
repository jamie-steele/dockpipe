#!/usr/bin/env bash
# Multi-step workflow: outputs.env feeds the next container; workflow runs go test in repo (--workdir).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLI="${REPO_ROOT}/bin/dockpipe"

test_chain_outputs() {
  local tmp mod
  tmp="$(mktemp -d)"
  pkg="$(cd "$REPO_ROOT" && go env GOPATH)/pkg"
  (cd "$REPO_ROOT" && go mod download)
  (
    cd "$tmp"
    DOCKPIPE_REPO_ROOT="$REPO_ROOT" "$CLI" --workflow test --workdir "$REPO_ROOT" \
      --mount "${pkg}:/go/pkg:rw" \
      2>&1 | tee "$tmp/out.log"
    if ! grep -q "pipeline complete" "$tmp/out.log"; then
      echo "test_chain_outputs FAIL: expected pipeline complete line in output"
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
