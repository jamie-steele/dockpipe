#!/usr/bin/env bash
# Test print-summary action: run in a git repo, command makes a change; summary shows exit code and uncommitted changes.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE="$REPO_ROOT/src/bin/dockpipe"
ACTION="$REPO_ROOT/src/core/assets/scripts/print-summary.sh"
source "$(dirname "${BASH_SOURCE[0]}")/helpers.sh"

require_agent_dev_template "$REPO_ROOT"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"
git init -q
git config user.email "test@dockpipe"
git config user.name "Test"
echo "initial" > initial.txt
git add initial.txt
git commit -q -m "initial"

out=$(mktemp)
trap 'rm -f "$out"' RETURN
"$DOCKPIPE" --no-data --template agent-dev --workdir "$tmp" --action "$ACTION" \
  -- sh -c 'echo "new" > newfile.txt' >"$out" 2>&1 || true

grep -q "\[dockpipe summary\]" "$out" || { echo "Expected [dockpipe summary] in output"; cat "$out"; exit 1; }
grep -q "Uncommitted changes" "$out" || { echo "Expected Uncommitted changes in summary"; cat "$out"; exit 1; }

echo "test_print_summary_action OK"
