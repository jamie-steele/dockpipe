#!/usr/bin/env bash
# Run unit tests. Exit 0 if all pass. From repo root: bash tests/run_tests.sh
# Integration tests (Docker): bash tests/integration-tests/run.sh
# Maintainer package tests live under packages/*/tests/ (self-contained).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UNIT="$(cd "${ROOT}/tests/unit-tests" && pwd)"
failed=0

for f in test_cli.sh test_repo_root.sh test_repo_layout.sh test_clone_worktree_include.sh; do
  if [[ -f "$UNIT/$f" ]]; then
    echo "--- $f ---"
    bash "$UNIT/$f" || failed=1
  fi
done

echo "--- maintainer package tests (pipeon, dorkpipe, dockpipe-mcp) ---"
bash "$ROOT/packages/pipeon/tests/run.sh" || failed=1
bash "$ROOT/packages/dorkpipe/tests/run.sh" || failed=1
bash "$ROOT/packages/dockpipe-mcp/tests/run.sh" || failed=1

echo "--- smoke.sh (needs Docker) ---"
bash "$UNIT/smoke.sh" || failed=1

echo "--- test_deb_install.sh (needs Docker + .deb) ---"
_deb="$(echo "$ROOT/release/packaging/build"/dockpipe_*_amd64.deb 2>/dev/null)"
_can_docker=0
if command -v docker &>/dev/null && docker run --rm debian:bookworm-slim true &>/dev/null; then
  _can_docker=1
fi
if [[ $_can_docker -eq 1 ]] && [[ -n "${_deb}" ]] && [[ -f "${_deb}" ]]; then
  bash "$UNIT/test_deb_install.sh" || failed=1
else
  echo "  (Docker runnable + .deb required; run ./release/packaging/build-deb.sh and ensure 'docker run' works to run this test)"
fi

if [[ $failed -eq 0 ]]; then
  echo "All tests passed."
else
  echo "Some tests failed."
  exit 1
fi
