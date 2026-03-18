#!/usr/bin/env bash
# Action: commit all changes in the current work directory (e.g. after running a command/script).
# Uses: DOCKPIPE_EXIT_CODE, DOCKPIPE_CONTAINER_WORKDIR. Optional: DOCKPIPE_COMMIT_MESSAGE, DOCKPIPE_COMMIT_BRANCH.
# Never commits on main/master: if current branch is main or master, creates a new branch (e.g. dockpipe/agent-<timestamp>)
# and commits there so the main branch is left unchanged.
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

current_branch="$(git branch --show-current)"
if [[ "$current_branch" == "main" || "$current_branch" == "master" ]]; then
  commit_branch="${DOCKPIPE_COMMIT_BRANCH:-dockpipe/agent-$(date +%Y%m%d-%H%M%S)}"
  echo "[dockpipe action] On $current_branch; creating branch '$commit_branch' so we do not commit on main." >&2
  git checkout -b "${commit_branch}"
fi

MSG="${DOCKPIPE_COMMIT_MESSAGE:-dockpipe: automated commit}"
git add -A
git commit -m "${MSG}"
echo "[dockpipe action] Committed: $(git log -1 --format='%H')" >&2
exit "$DOCKPIPE_EXIT_CODE"
