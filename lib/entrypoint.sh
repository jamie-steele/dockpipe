#!/usr/bin/env bash
# dockpipe container entrypoint.
# Runs the user command; if DOCKPIPE_ACTION is set, runs that script after the command exits
# with DOCKPIPE_EXIT_CODE and DOCKPIPE_CONTAINER_WORKDIR set. On SIGTERM/SIGHUP/SIGINT we
# terminate the command and exit so the container does not linger.
set -euo pipefail

WORKDIR="${DOCKPIPE_CONTAINER_WORKDIR:-/work}"
cd "${WORKDIR}"

# Git identity: container can't see host's .gitconfig. Pass from host: --env "GIT_AUTHOR_EMAIL=$(git config user.email)" --env "GIT_AUTHOR_NAME=$(git config user.name)"
if [[ -n "${GIT_AUTHOR_EMAIL:-}" ]]; then git config --global user.email "${GIT_AUTHOR_EMAIL}"; fi
if [[ -n "${GIT_AUTHOR_NAME:-}" ]]; then git config --global user.name "${GIT_AUTHOR_NAME}"; fi

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
