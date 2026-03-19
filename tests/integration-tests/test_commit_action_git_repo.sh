#!/usr/bin/env bash
# Integration test: temp git repo, dockpipe with commit-worktree action and a command that creates a file; assert action commits.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE="$REPO_ROOT/bin/dockpipe"
ACTION="$REPO_ROOT/scripts/commit-worktree.sh"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"

git init -q
git config user.email "test@dockpipe"
git config user.name "Test"
echo "initial" > initial.txt
git add initial.txt
git commit -q -m "initial"

# Run dockpipe: command creates a new file, action should commit it
"$DOCKPIPE" --template agent-dev --workdir "$tmp" --action "$ACTION" \
  --env "DOCKPIPE_COMMIT_MESSAGE=agent: integration test" \
  -- sh -c 'echo "from container" > newfile.txt'

# Assert: newfile.txt exists and was committed
[[ -f "$tmp/newfile.txt" ]] || { echo "Expected newfile.txt in repo"; exit 1; }
[[ "$(cat "$tmp/newfile.txt")" == "from container" ]] || { echo "Expected content in newfile.txt"; exit 1; }
git -C "$tmp" log -1 --format=%s | grep -q "agent: integration test" || { echo "Expected commit message"; exit 1; }
git -C "$tmp" show --name-only --format= HEAD | grep -q newfile.txt || { echo "Expected newfile.txt in last commit"; exit 1; }

echo "test_commit_action_git_repo OK"
