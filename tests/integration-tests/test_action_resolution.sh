#!/usr/bin/env bash
# Test that --action scripts/commit-worktree.sh resolves to the bundled script when run from outside the repo.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE="$REPO_ROOT/bin/dockpipe"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"

git init -q
git config user.email "test@dockpipe"
git config user.name "Test"
echo "initial" > initial.txt
git add initial.txt
git commit -q -m "initial"

# Run from $tmp with relative action path (must resolve to bundled templates/core/assets/scripts/commit-worktree.sh)
"$DOCKPIPE" --workdir "$tmp" --action scripts/commit-worktree.sh \
  --env "DOCKPIPE_COMMIT_MESSAGE=resolution test" \
  -- sh -c 'echo "resolved" > resolved.txt'

[[ -f "$tmp/resolved.txt" ]] || { echo "Expected resolved.txt"; exit 1; }
git -C "$tmp" log -1 --format=%s | grep -q "resolution test" || { echo "Expected commit message"; exit 1; }

echo "test_action_resolution OK"
