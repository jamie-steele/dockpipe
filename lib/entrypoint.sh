#!/usr/bin/env bash
# Generic container entrypoint for dockpipe images.
# 1. Run the command passed as argv (or from DOCKPIPE_CMD).
# 2. If DOCKPIPE_ACTION is set, run that script with DOCKPIPE_EXIT_CODE and DOCKPIPE_CONTAINER_WORKDIR.
# When the terminal closes or the client disconnects, Docker sends SIGTERM; we kill the command and exit
# so the container comes down instead of lingering.
set -euo pipefail

WORKDIR="${DOCKPIPE_CONTAINER_WORKDIR:-/work}"
cd "${WORKDIR}"

CMD_PID=""
cleanup() {
  if [[ -n "${CMD_PID}" ]]; then
    kill -TERM "${CMD_PID}" 2>/dev/null || true
    wait "${CMD_PID}" 2>/dev/null || true
  fi
  exit 130
}
trap cleanup SIGTERM SIGHUP SIGINT

# Run the user's command in background so we can forward signals (e.g. on terminal close)
if [[ $# -gt 0 ]]; then
  "$@" &
else
  if [[ -n "${DOCKPIPE_CMD:-}" ]]; then
    eval "${DOCKPIPE_CMD}" &
  else
    echo "dockpipe: no command given" >&2
    exit 1
  fi
fi
CMD_PID=$!
wait "${CMD_PID}"
trap - SIGTERM SIGHUP SIGINT
DOCKPIPE_EXIT_CODE=$?

# Run action script if present (e.g. commit-worktree, export-patch)
if [[ -n "${DOCKPIPE_ACTION:-}" ]] && [[ -f "${DOCKPIPE_ACTION}" ]]; then
  export DOCKPIPE_EXIT_CODE DOCKPIPE_CONTAINER_WORKDIR
  set +e
  bash "${DOCKPIPE_ACTION}"
  set -e
fi

exit "$DOCKPIPE_EXIT_CODE"
