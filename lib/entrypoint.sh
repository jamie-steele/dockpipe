#!/usr/bin/env bash
# Generic container entrypoint for dockpipe images.
# 1. Run the command passed as argv (or from DOCKPIPE_CMD).
# 2. If DOCKPIPE_ACTION is set, run that script with DOCKPIPE_EXIT_CODE and DOCKPIPE_CONTAINER_WORKDIR.
set -euo pipefail

WORKDIR="${DOCKPIPE_CONTAINER_WORKDIR:-/work}"
cd "${WORKDIR}"

# Run the user's command (everything passed as args, or single shell command from env)
if [[ $# -gt 0 ]]; then
  "$@"
else
  if [[ -n "${DOCKPIPE_CMD:-}" ]]; then
    eval "${DOCKPIPE_CMD}"
  else
    echo "dockpipe: no command given" >&2
    exit 1
  fi
fi
DOCKPIPE_EXIT_CODE=$?

# Run action script if present (e.g. commit-worktree, export-patch)
if [[ -n "${DOCKPIPE_ACTION:-}" ]] && [[ -f "${DOCKPIPE_ACTION}" ]]; then
  export DOCKPIPE_EXIT_CODE DOCKPIPE_CONTAINER_WORKDIR
  set +e
  bash "${DOCKPIPE_ACTION}"
  set -e
fi

exit "$DOCKPIPE_EXIT_CODE"
