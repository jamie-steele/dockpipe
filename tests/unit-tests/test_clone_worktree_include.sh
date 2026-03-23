#!/usr/bin/env bash
# Unit test: clone-worktree.sh copies gitignored paths listed in .dockpipe-worktreeinclude
# and skips .worktreeinclude when format != 1. Requires bash 4+, git; no Docker.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/src/templates/core/assets/scripts/clone-worktree.sh"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

setup_repo() {
  local seed="$1"
  mkdir -p "$seed"
  git -C "$seed" init -q -b main
  git -C "$seed" config user.email "test@dockpipe"
  git -C "$seed" config user.name "Test"
  echo "*.local" >>"$seed/.gitignore"
  echo "v=1" >"$seed/foo.local"
  echo "x" >"$seed/README.md"
  git -C "$seed" add .gitignore README.md
  git -C "$seed" commit -q -m seed
}

run_clone_worktree() {
  local seed="$1" data_dir="$2" branch="$3" bare="$4"
  export DOCKPIPE_REPO_URL="file://$bare"
  export DOCKPIPE_REPO_BRANCH="$branch"
  export DOCKPIPE_USER_REPO_ROOT="$seed"
  export DOCKPIPE_DATA_DIR="$data_dir"
  unset DOCKPIPE_WORKDIR
  # shellcheck disable=SC1090
  source "$SCRIPT"
}

# --- Test 1: .dockpipe-worktreeinclude copies gitignored foo.local ---
seed="$tmp/s1"
setup_repo "$seed"
bare="$tmp/r1.git"
git init --bare -q "$bare"
git -C "$seed" remote add origin "file://$bare"
git -C "$seed" push -q -u origin main
git -C "$bare" symbolic-ref HEAD refs/heads/main

echo "foo.local" >"$seed/.dockpipe-worktreeinclude"
branch="dockpipe-it-inc-1"
run_clone_worktree "$seed" "$tmp/data1" "$branch" "$bare"

[[ -n "${DOCKPIPE_WORKDIR:-}" ]] || {
  echo "expected DOCKPIPE_WORKDIR"
  exit 1
}
[[ -f "$DOCKPIPE_WORKDIR/foo.local" ]] || {
  echo "expected foo.local in worktree from include"
  exit 1
}
[[ "$(cat "$DOCKPIPE_WORKDIR/foo.local")" == "v=1" ]] || {
  echo "foo.local content mismatch"
  exit 1
}
[[ -f "$seed/foo.local" ]] || {
  echo "main checkout should still have foo.local"
  exit 1
}

# --- Test 2: format 2 skips file (no copy from .worktreeinclude) ---
seed2="$tmp/s2"
setup_repo "$seed2"
bare2="$tmp/r2.git"
git init --bare -q "$bare2"
git -C "$seed2" remote add origin "file://$bare2"
git -C "$seed2" push -q -u origin main
git -C "$bare2" symbolic-ref HEAD refs/heads/main

{
  echo "# dockpipe-worktreeinclude-format: 2"
  echo "foo.local"
} >"$seed2/.worktreeinclude"
branch2="dockpipe-it-inc-2"
run_clone_worktree "$seed2" "$tmp/data2" "$branch2" "$bare2"

[[ -n "${DOCKPIPE_WORKDIR:-}" ]] || {
  echo "expected DOCKPIPE_WORKDIR (test2)"
  exit 1
}
if [[ -f "$DOCKPIPE_WORKDIR/foo.local" ]]; then
  echo "did not expect foo.local when format 2 skipped"
  exit 1
fi

# --- Test 3: .dockpipe-worktreeinclude wins over .worktreeinclude ---
seed3="$tmp/s3"
setup_repo "$seed3"
echo "other" >"$seed3/bar.local"
bare3="$tmp/r3.git"
git init --bare -q "$bare3"
git -C "$seed3" remote add origin "file://$bare3"
git -C "$seed3" push -q -u origin main
git -C "$bare3" symbolic-ref HEAD refs/heads/main

echo "bar.local" >"$seed3/.worktreeinclude"
echo "foo.local" >"$seed3/.dockpipe-worktreeinclude"
branch3="dockpipe-it-inc-3"
run_clone_worktree "$seed3" "$tmp/data3" "$branch3" "$bare3"

[[ -f "$DOCKPIPE_WORKDIR/foo.local" ]] || {
  echo "expected dockpipe file to include foo.local"
  exit 1
}
if [[ -f "$DOCKPIPE_WORKDIR/bar.local" ]]; then
  echo "did not expect bar.local (only .dockpipe-worktreeinclude should apply)"
  exit 1
fi

echo "test_clone_worktree_include OK"
