#!/usr/bin/env bash
# Run unit tests. Exit 0 if all pass. From repo root: bash tests/run_tests.sh
# Integration tests (Docker): bash tests/integration-tests/run.sh
# smoke.sh counts toward pass/fail when Docker is available (exits 0 if Docker missing).
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/unit-tests" && pwd)"
failed=0

for f in test_cli.sh test_runner.sh test_repo_root.sh; do
  if [[ -f "$DIR/$f" ]]; then
    echo "--- $f ---"
    bash "$DIR/$f" || failed=1
  fi
done

echo "--- smoke.sh (needs Docker) ---"
bash "$DIR/smoke.sh" || failed=1

echo "--- test_deb_install.sh (needs Docker + .deb) ---"
_deb="$(echo "$DIR/../../packaging/build"/dockpipe_*_amd64.deb 2>/dev/null)"
_can_docker=0
if command -v docker &>/dev/null && docker run --rm debian:bookworm-slim true &>/dev/null; then
  _can_docker=1
fi
if [[ $_can_docker -eq 1 ]] && [[ -n "${_deb}" ]] && [[ -f "${_deb}" ]]; then
  bash "$DIR/test_deb_install.sh" || failed=1
else
  echo "  (Docker runnable + .deb required; run ./packaging/build-deb.sh and ensure 'docker run' works to run this test)"
fi

if [[ $failed -eq 0 ]]; then
  echo "All tests passed."
else
  echo "Some tests failed."
  exit 1
fi
