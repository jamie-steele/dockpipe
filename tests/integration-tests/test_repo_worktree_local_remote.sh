#!/usr/bin/env bash
# Integration test: --repo/--branch with a local bare remote should create and use
# a host worktree under --data-dir with no network credentials.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE="$REPO_ROOT/src/bin/dockpipe"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

seed_repo="$tmp/seed"
remote_bare="$tmp/remote.git"
data_dir="$tmp/data"
branch="dockpipe/it-local"

mkdir -p "$seed_repo"
git -C "$seed_repo" init -q -b main
git -C "$seed_repo" config user.email "test@dockpipe"
git -C "$seed_repo" config user.name "Test"
echo "seed" > "$seed_repo/README.md"
git -C "$seed_repo" add README.md
git -C "$seed_repo" commit -q -m "seed"

git init --bare -q "$remote_bare"
git -C "$seed_repo" remote add origin "$remote_bare"
git -C "$seed_repo" push -q -u origin main

# Ensure the bare remote advertises HEAD -> main (clone-worktree.sh reads this).
git -C "$remote_bare" symbolic-ref HEAD refs/heads/main

remote_url="file://$remote_bare"
"$DOCKPIPE" --repo "$remote_url" --branch "$branch" --data-dir "$data_dir" \
  -- sh -c 'echo "from container" > from_container.txt'

repo_name="$(basename "$remote_bare" .git)"
safe_branch="${branch//\//-}"
worktree_path="$data_dir/repos/$repo_name/worktrees/$safe_branch"

[[ -e "$worktree_path/.git" ]] || { echo "Expected worktree at $worktree_path"; exit 1; }
[[ -f "$worktree_path/from_container.txt" ]] || { echo "Expected from_container.txt in worktree"; exit 1; }
[[ "$(cat "$worktree_path/from_container.txt")" == "from container" ]] || { echo "Expected from_container.txt content"; exit 1; }
[[ "$(git -C "$worktree_path" branch --show-current)" == "$branch" ]] || { echo "Expected branch $branch in worktree"; exit 1; }

# Re-run to ensure existing worktree path is reusable.
"$DOCKPIPE" --repo "$remote_url" --branch "$branch" --data-dir "$data_dir" \
  -- sh -c 'echo "second run" > second_run.txt'
[[ -f "$worktree_path/second_run.txt" ]] || { echo "Expected second_run.txt after reuse"; exit 1; }

echo "test_repo_worktree_local_remote OK"

