#!/usr/bin/env bash
# Action: print a short summary of the run (exit code, git status if repo).
set -euo pipefail

echo "[dockpipe summary] Exit code: ${DOCKPIPE_EXIT_CODE}" >&2
cd "${DOCKPIPE_CONTAINER_WORKDIR:-/work}"

if git rev-parse --is-inside-work-tree &>/dev/null; then
  echo "[dockpipe summary] Branch: $(git branch --show-current)" >&2
  echo "[dockpipe summary] Last commit: $(git log -1 --format='%h %s' 2>/dev/null || echo 'none')" >&2
  if ! git diff --quiet HEAD 2>/dev/null || [[ -n "$(git status --porcelain)" ]]; then
    echo "[dockpipe summary] Uncommitted changes: yes" >&2
  else
    echo "[dockpipe summary] Uncommitted changes: no" >&2
  fi
fi
exit "$DOCKPIPE_EXIT_CODE"
