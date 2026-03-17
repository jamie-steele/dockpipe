#!/usr/bin/env bash
# Action: commit all changes in the current work directory (e.g. after running a command/script).
# Uses: DOCKPIPE_EXIT_CODE, DOCKPIPE_CONTAINER_WORKDIR. Optional: DOCKPIPE_COMMIT_MESSAGE, DOCKPIPE_COMMIT_BRANCH.
# Use with a repo worktree; commit is left in the worktree for cherry-pick or push.
set -euo pipefail

cd "${DOCKPIPE_CONTAINER_WORKDIR:-/work}"

if ! git rev-parse --is-inside-work-tree &>/dev/null; then
  echo "[dockpipe action] Not a git repo; skipping commit." >&2
  exit "$DOCKPIPE_EXIT_CODE"
fi

if git diff --quiet HEAD 2>/dev/null && [[ -z "$(git status --porcelain)" ]]; then
  echo "[dockpipe action] No changes to commit." >&2
  exit "$DOCKPIPE_EXIT_CODE"
fi

MSG="${DOCKPIPE_COMMIT_MESSAGE:-dockpipe: automated commit}"
git add -A
git commit -m "${MSG}"
echo "[dockpipe action] Committed: $(git log -1 --format='%H')" >&2
exit "$DOCKPIPE_EXIT_CODE"
