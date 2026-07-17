#!/usr/bin/env bash
# Multi-step workflow: outputs.env feeds the next container; workflow runs go test in repo (--workdir).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLI="${REPO_ROOT}/src/bin/dockpipe"

test_chain_outputs() {
  local tmp mod
  tmp="$(mktemp -d)"
  pkg="$(cd "$REPO_ROOT" && go env GOPATH)/pkg"
  local -a extra_args=()
  case "${OS:-}" in
    Windows_NT)
      extra_args+=(--var DOCKPIPE_DOCKER_NETWORK=bridge)
      ;;
  esac
  (cd "$REPO_ROOT" && go mod download)
  (
    cd "$tmp"
    DOCKPIPE_REPO_ROOT="$REPO_ROOT" \
      "$CLI" --workflow test --workdir "$REPO_ROOT" \
        "${extra_args[@]}" \
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
  case "${OS:-}" in
    Windows_NT)
      echo "test_chain_outputs SKIP: Windows host Git Bash path is covered by ci-emulate host pre-run; workflow test remains canonical on Linux CI"
      return 0
      ;;
  esac
  test_chain_outputs
}

run_tests
