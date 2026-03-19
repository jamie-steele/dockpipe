#!/usr/bin/env bash
# Test print-summary action: run in a git repo, command makes a change; summary shows exit code and uncommitted changes.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE="$REPO_ROOT/bin/dockpipe"
ACTION="$REPO_ROOT/scripts/print-summary.sh"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"
git init -q
git config user.email "test@dockpipe"
git config user.name "Test"
echo "initial" > initial.txt
git add initial.txt
git commit -q -m "initial"

stderr=$(mktemp)
trap 'rm -f "$stderr"' RETURN
"$DOCKPIPE" --no-data --template agent-dev --workdir "$tmp" --action "$ACTION" \
  -- sh -c 'echo "new" > newfile.txt' 2>"$stderr" || true

grep -q "\[dockpipe summary\]" "$stderr" || { echo "Expected [dockpipe summary] in stderr"; cat "$stderr"; exit 1; }
grep -q "Uncommitted changes" "$stderr" || { echo "Expected Uncommitted changes in summary"; cat "$stderr"; exit 1; }

echo "test_print_summary_action OK"
