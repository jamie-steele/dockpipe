#!/usr/bin/env bash
# Test dockpipe action init (boilerplate) and action init --from <bundled>.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE="$REPO_ROOT/src/bin/dockpipe"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"

# 1. Boilerplate init
"$DOCKPIPE" action init my-action.sh
[[ -f "$tmp/my-action.sh" ]] || { echo "Expected my-action.sh"; exit 1; }
[[ -x "$tmp/my-action.sh" ]] || { echo "Expected executable"; exit 1; }
grep -q "DOCKPIPE_EXIT_CODE" "$tmp/my-action.sh" || { echo "Expected boilerplate content"; exit 1; }

# 2. Clone bundled action
"$DOCKPIPE" action init my-commit.sh --from commit-worktree
[[ -f "$tmp/my-commit.sh" ]] || { echo "Expected my-commit.sh"; exit 1; }
[[ -x "$tmp/my-commit.sh" ]] || { echo "Expected executable"; exit 1; }
grep -q "commit-on-host" "$tmp/my-commit.sh" && grep -q "Not a git repo; skipping commit" "$tmp/my-commit.sh" && grep -q "DOCKPIPE" "$tmp/my-commit.sh" || { echo "Expected my-commit.sh to contain commit-worktree logic"; head -20 "$tmp/my-commit.sh"; exit 1; }

# 3. Clone with --from first (arg order)
"$DOCKPIPE" action init --from print-summary my-summary.sh
[[ -f "$tmp/my-summary.sh" ]] || { echo "Expected my-summary.sh"; exit 1; }
grep -q "DOCKPIPE_EXIT_CODE" "$tmp/my-summary.sh" || { echo "Expected print-summary content"; exit 1; }

echo "test_action_init OK"
