#!/usr/bin/env bash
# Run all tests. Exit 0 if all pass.
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
failed=0

for f in test_cli.sh test_runner.sh; do
  if [[ -f "$DIR/$f" ]]; then
    echo "--- $f ---"
    bash "$DIR/$f" || failed=1
  fi
done

echo "--- smoke.sh (optional, needs Docker) ---"
bash "$DIR/smoke.sh" || true

if [[ $failed -eq 0 ]]; then
  echo "All tests passed."
else
  echo "Some tests failed."
  exit 1
fi
