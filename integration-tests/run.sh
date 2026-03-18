#!/usr/bin/env bash
# Run integration tests. Require Docker. Run from repo root: bash integration-tests/run.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
failed=0

if ! command -v docker &>/dev/null; then
  echo "Error: docker not in PATH. Integration tests require Docker." >&2
  exit 1
fi
if ! docker run --rm debian:bookworm-slim true &>/dev/null; then
  echo "Error: docker run failed. Ensure Docker is running and you can run containers." >&2
  exit 1
fi

for f in "$DIR"/test_*.sh; do
  [[ -f "$f" ]] || continue
  name="$(basename "$f")"
  echo "--- $name ---"
  bash "$f" || failed=1
done

if [[ $failed -eq 0 ]]; then
  echo "All integration tests passed."
else
  echo "Some integration tests failed."
  exit 1
fi
