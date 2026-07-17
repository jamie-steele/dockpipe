#!/bin/bash
# dockpipe container entrypoint.
# Runs the user command; if DOCKPIPE_ACTION is set, runs that script after the command exits
# with DOCKPIPE_EXIT_CODE and DOCKPIPE_CONTAINER_WORKDIR set. On SIGTERM/SIGHUP/SIGINT we
# terminate the command and exit so the container does not linger.
set -euo pipefail
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin${PATH:+:$PATH}"

WORKDIR="${DOCKPIPE_CONTAINER_WORKDIR:-/work}"
cd "${WORKDIR}"

# DOCKPIPE_DEBUG=1 logs uid before your command.
if [[ "${DOCKPIPE_DEBUG:-}" == "1" ]]; then
  _du="$(id -u)" _dg="$(id -g)"
  _dn="$(awk -F: -v u="$_du" '$3 == u { print $1; exit }' /etc/passwd 2>/dev/null)"
  [[ -z "${_dn:-}" ]] && _dn="$_du"
  echo "[dockpipe] entrypoint: uid=${_du} gid=${_dg} name=${_dn} argv0=$0" >&2
  unset _du _dg _dn
fi

# Normalize USER/LOGNAME when the runtime only provides a numeric uid.
_uid="$(id -u)"
if _name="$(awk -F: -v u="$_uid" '$3 == u { print $1; exit }' /etc/passwd 2>/dev/null)" && [[ -n "${_name:-}" ]]; then
  export USER="${_name}" LOGNAME="${_name}"
else
  export USER="${_uid}" LOGNAME="${_uid}"
fi
unset _uid _name

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

# No action: run command in foreground. Interactive shell wrapping is opt-in; non-interactive
# workflow workers must not get bash job-control noise or inherited stdin behavior.
# With action: run command in background, wait, then run the action script.
if [[ -z "${DOCKPIPE_ACTION:-}" ]] || [[ ! -f "${DOCKPIPE_ACTION:-}" ]]; then
  if [[ $# -gt 0 ]]; then
    if [[ "${DOCKPIPE_INTERACTIVE_ENTRYPOINT:-auto}" =~ ^(1|true|yes|on)$ ]] || { [[ "${DOCKPIPE_INTERACTIVE_ENTRYPOINT:-auto}" == "auto" ]] && [[ -t 0 && -t 1 ]]; }; then
      bash -i -c 'exec "$@"' _ "$@"
    else
      exec "$@"
    fi
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
