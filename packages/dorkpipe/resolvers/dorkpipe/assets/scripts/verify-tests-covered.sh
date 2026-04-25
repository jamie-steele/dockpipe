#!/usr/bin/env bash
# Stub: ensure changed files from git diff have matching *_test.go (heuristic).
set -euo pipefail
repo="${1:-.}"
mapfile -t files < <(git -C "$repo" diff --name-only HEAD 2>/dev/null || true)
if ((${#files[@]} == 0)); then
  echo "verify-tests-covered: no diff; ok"
  exit 0
fi
missing=0
for f in "${files[@]}"; do
  [[ "$f" != *.go ]] && continue
  [[ "$f" == *_test.go ]] && continue
  base="${f%.go}"
  if [[ ! -f "${base}_test.go" ]]; then
    echo "verify-tests-covered: no test for $f" >&2
    missing=1
  fi
done
exit "$missing"
