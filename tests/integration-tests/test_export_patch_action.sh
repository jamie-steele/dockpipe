#!/usr/bin/env bash
# Test export-patch action: command creates a new file; action writes dockpipe.patch; assert patch exists and contains the diff.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE="$REPO_ROOT/src/bin/dockpipe"
ACTION="$REPO_ROOT/src/core/assets/scripts/export-patch.sh"
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

"$DOCKPIPE" --no-data --template agent-dev --workdir "$tmp" --action "$ACTION" \
  -- sh -c 'echo "patched line" > newfile.txt && git add newfile.txt'

[[ -f "$tmp/dockpipe.patch" ]] || { echo "Expected dockpipe.patch"; exit 1; }
[[ -s "$tmp/dockpipe.patch" ]] || { echo "Expected non-empty patch"; cat "$tmp/dockpipe.patch"; exit 1; }
grep -q "newfile" "$tmp/dockpipe.patch" || { echo "Expected newfile in patch"; exit 1; }
grep -q "patched" "$tmp/dockpipe.patch" || { echo "Expected patch to contain new content"; cat "$tmp/dockpipe.patch"; exit 1; }

echo "test_export_patch_action OK"
