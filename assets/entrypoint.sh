#!/usr/bin/env bash
# dockpipe container entrypoint.
# Runs the user command; if DOCKPIPE_ACTION is set, runs that script after the command exits
# with DOCKPIPE_EXIT_CODE and DOCKPIPE_CONTAINER_WORKDIR set. On SIGTERM/SIGHUP/SIGINT we
# terminate the command and exit so the container does not linger.
set -euo pipefail

WORKDIR="${DOCKPIPE_CONTAINER_WORKDIR:-/work}"
cd "${WORKDIR}"

# Some hosts (Docker Desktop on Windows, etc.) start the container as root even when the image sets
# USER node. Claude Code rejects --dangerously-skip-permissions for root — re-exec as "node" without
# needing docker run -u (avoids bind-mount stalls). Opt out: DOCKPIPE_SKIP_DROP_TO_NODE=1.
# DOCKPIPE_DEBUG=1 logs uid before your command.
if [[ "${DOCKPIPE_DEBUG:-}" == "1" ]]; then
  echo "[dockpipe] entrypoint: uid=$(id -u) gid=$(id -g) name=$(id -un) argv0=$0" >&2
fi

if [[ -z "${DOCKPIPE_SKIP_DROP_TO_NODE:-}" ]] && [[ "$(id -u)" -eq 0 ]] && [[ -z "${DOCKPIPE_ENTRYPOINT_DROPPED:-}" ]] && id -u node &>/dev/null; then
  export DOCKPIPE_ENTRYPOINT_DROPPED=1
  _ep=/entrypoint.sh
  [[ -f "${_ep}" ]] || _ep="$0"
  echo "[dockpipe] entrypoint: switching root → user node (Claude / permission flags)" >&2
  if command -v runuser >/dev/null 2>&1; then
    exec runuser -u node -- "${_ep}" "$@"
  fi
  if command -v setpriv >/dev/null 2>&1; then
    exec setpriv --reuid="$(id -u node)" --regid="$(id -g node)" --init-groups -- "${_ep}" "$@"
  fi
  echo "[dockpipe] entrypoint: FATAL: need runuser or setpriv (util-linux) to drop root → node" >&2
  exit 1
fi

# Claude Code treats sudo-like env as elevated; clear if leaked from host -e passthrough.
unset SUDO_COMMAND SUDO_USER SUDO_UID SUDO_GID 2>/dev/null || true

# Some CLIs treat USER=root (or stale LOGNAME) as elevated even when uid is non-root (e.g. Docker defaults).
export USER="$(id -un)"
export LOGNAME="$(id -un)"

# Claude Code: --dangerously-skip-permissions is allowed when IS_SANDBOX=1 (disposable Docker / devcontainers).
# https://github.com/anthropics/claude-code/issues/9184 — without this, root-detection blocks the flag even in real sandboxes.
# Respect host -e IS_SANDBOX if already set; opt out: DOCKPIPE_NO_SANDBOX_ENV=1
if [[ -z "${DOCKPIPE_NO_SANDBOX_ENV:-}" ]] && [[ -z "${IS_SANDBOX:-}" ]]; then
  export IS_SANDBOX=1
fi

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
