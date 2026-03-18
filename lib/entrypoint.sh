#!/usr/bin/env bash
# dockpipe container entrypoint.
# Runs the user command; if DOCKPIPE_ACTION is set, runs that script after the command exits
# with DOCKPIPE_EXIT_CODE and DOCKPIPE_CONTAINER_WORKDIR set. On SIGTERM/SIGHUP/SIGINT we
# terminate the command and exit so the container does not linger.
set -euo pipefail

WORKDIR="${DOCKPIPE_CONTAINER_WORKDIR:-/work}"
cd "${WORKDIR}"

# Diagnostic lines shown when runner dumps logs (e.g. "Container exited quickly").
echo "dockpipe entrypoint: argc=$# argv=($*)" >&2
echo "dockpipe entrypoint: stdin is TTY=$([ -t 0 ] && echo yes || echo no)" >&2

CMD_PID=""
cleanup() {
  if [[ -n "${CMD_PID}" ]]; then
    kill -TERM "${CMD_PID}" 2>/dev/null || true
    wait "${CMD_PID}" 2>/dev/null || true
  fi
  exit 130
}
trap cleanup SIGTERM SIGHUP SIGINT

# No action: run command in foreground so it keeps the TTY (required for interactive CLIs).
# With action: run command in background, wait, then run the action script.
if [[ -z "${DOCKPIPE_ACTION:-}" ]] || [[ ! -f "${DOCKPIPE_ACTION:-}" ]]; then
  if [[ $# -gt 0 ]]; then
    bash -i -c 'exec "$@"' _ "$@" || true
    _rc=$?
    echo "dockpipe entrypoint: command exited with code $_rc" >&2
    exit "$_rc"
  else
    if [[ -n "${DOCKPIPE_CMD:-}" ]]; then
      exec bash -c "${DOCKPIPE_CMD}"
    else
      echo "dockpipe: no command given" >&2
      exit 1
    fi
  fi
fi

if [[ $# -gt 0 ]]; then
  "$@" &
else
  eval "${DOCKPIPE_CMD}" &
fi
CMD_PID=$!
wait "${CMD_PID}"
trap - SIGTERM SIGHUP SIGINT
DOCKPIPE_EXIT_CODE=$?

export DOCKPIPE_EXIT_CODE DOCKPIPE_CONTAINER_WORKDIR
set +e
bash "${DOCKPIPE_ACTION}"
set -e

exit "$DOCKPIPE_EXIT_CODE"
