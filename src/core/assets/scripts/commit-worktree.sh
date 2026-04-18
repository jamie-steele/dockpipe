#!/usr/bin/env bash
# Action: commit all changes in the current work directory (e.g. after running a command/script).
# When commit-on-host is enabled, the real commit runs on the host after container exit.
set -euo pipefail

cd "${DOCKPIPE_CONTAINER_WORKDIR:-/work}"

if ! git rev-parse --is-inside-work-tree &>/dev/null; then
  echo "[dockpipe action] Not a git repo; skipping commit." >&2
  exit "${DOCKPIPE_EXIT_CODE:-1}"
fi

if git diff --quiet HEAD 2>/dev/null && [[ -z "$(git status --porcelain)" ]]; then
  echo "[dockpipe action] No changes to commit." >&2
  exit "${DOCKPIPE_EXIT_CODE:-0}"
fi

# When commit-on-host is used, dockpipe runs the commit on the host; this script just exits.
exit "${DOCKPIPE_EXIT_CODE:-0}"
