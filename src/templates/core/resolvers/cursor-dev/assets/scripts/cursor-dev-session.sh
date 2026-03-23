#!/usr/bin/env bash
# Host-only: long-lived base-dev container (like vscode’s docker session) + Cursor on the host.
# Blocks until the container exits (docker stop NAME) or Ctrl+C (stops container then exits).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/cursor-dev-common.sh"

if [[ -f "${SCRIPT_DIR}/cursor-prep.sh" ]]; then
  bash "${SCRIPT_DIR}/cursor-prep.sh"
fi

cursor_dev_set_workdir
if ! cursor_dev_docker_preflight; then
  exit 1
fi

# Resolve materialized dockpipe root for docker build. DOCKPIPE_REPO_ROOT from the Go binary may be a
# Windows path that breaks [[ -f ... ]] in Git Bash; script lives under templates/core/resolvers/cursor-dev/assets/scripts/.
cursor_dev_repo_root() {
  local r=""
  if [[ -n "${DOCKPIPE_REPO_ROOT:-}" ]]; then
    r="$(cd "$DOCKPIPE_REPO_ROOT" 2>/dev/null && pwd || true)"
  fi
  if [[ -z "$r" ]] || [[ ! -f "$r/templates/core/assets/images/base-dev/Dockerfile" ]]; then
    r="$(cd "$SCRIPT_DIR/../../../../../../" 2>/dev/null && pwd || true)"
  fi
  printf '%s' "$r"
}
REPO_ROOT="$(cursor_dev_repo_root)"

IMAGE="${CURSOR_DEV_SESSION_IMAGE:-dockpipe-base-dev:latest}"
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  if docker image inspect dockpipe-base-dev:latest >/dev/null 2>&1; then
    IMAGE=dockpipe-base-dev:latest
  elif docker image inspect dockpipe-base-dev >/dev/null 2>&1; then
    IMAGE=dockpipe-base-dev
  elif [[ -n "$REPO_ROOT" ]] && [[ -f "$REPO_ROOT/templates/core/assets/images/base-dev/Dockerfile" ]]; then
    printf '[dockpipe] Building dockpipe-base-dev image…\n' >&2
    docker build -q -t dockpipe-base-dev -f "$REPO_ROOT/templates/core/assets/images/base-dev/Dockerfile" "$REPO_ROOT"
    IMAGE=dockpipe-base-dev:latest
  else
    printf '[dockpipe] Image dockpipe-base-dev not found and could not build (no templates/core/assets/images/base-dev in %s).\n' "${REPO_ROOT:-<unknown>}" >&2
    printf '  Run: dockpipe --isolate base-dev -- echo ok   or set DOCKPIPE_REPO_ROOT to a full dockpipe layout.\n' >&2
    exit 1
  fi
fi

# Git Bash / MSYS maps "/work" to C:/Program Files/Git/work when invoking docker.exe. The "//work"
# trick is not reliable on all setups; MSYS2_ARG_CONV_EXCL=* disables path conversion for the whole
# docker invocation (see also vscode-code-server.sh comments).
cursor_dev_docker_no_pathconv() {
  if cursor_dev_is_msysish; then
    MSYS2_ARG_CONV_EXCL="*" docker "$@"
  else
    docker "$@"
  fi
}

NAME="${CURSOR_DEV_CONTAINER_NAME:-dockpipe-cursor-dev-${RANDOM}${RANDOM}}"

run_args=(
  run -d --rm --name "$NAME" --init --hostname dockpipe
  -v "${W}:/work"
  -w /work
  -e "DOCKPIPE_CONTAINER_WORKDIR=/work"
  -e "IS_SANDBOX=1"
)
case "${OSTYPE:-}" in
  linux-gnu*|darwin*)
    run_args+=(-u "$(id -u):$(id -g)")
    ;;
esac
# Override image ENTRYPOINT so we keep a single long-running process (session ends with docker stop or Ctrl+C).
# One line to stdout so Docker Desktop → Logs shows the session started (sleep infinity is silent).
run_args+=(--entrypoint /bin/bash "$IMAGE" -c 'printf "%s\n" "[dockpipe] cursor-dev session: idle, repo at /work"; exec sleep infinity')

cursor_dev_docker_no_pathconv "${run_args[@]}" >/dev/null

printf '\n[cursor-dev] Session container is running (same idea as vscode: dockpipe waits on Docker).\n' >&2
printf '  Name:     %s\n' "$NAME" >&2
printf '  Project:  %s  →  /work in the container\n' "$W" >&2
printf '  Stop:     docker stop %s   or press Ctrl+C here (stops the container).\n' "$NAME" >&2

cursor_dev_print_instructions

if try_launch_cursor; then
  :
else
  printf '\n[cursor-dev] Cursor CLI not found — open the folder above manually in Cursor.\n' >&2
fi

# When Cursor exits, stop the session container so docker wait returns.
# CURSOR_DEV_WAIT=0|none — only wait on docker (see templates/core/resolvers/cursor-dev/README.md).
CURSOR_DEV_WAIT="${CURSOR_DEV_WAIT:-1}"
CURSOR_BG_WAIT_PID=""

cleanup_session() {
  [[ -n "${CURSOR_BG_WAIT_PID:-}" ]] && kill "$CURSOR_BG_WAIT_PID" 2>/dev/null || true
  cursor_dev_docker_no_pathconv stop "$NAME" >/dev/null 2>&1 || true
}
trap cleanup_session INT TERM

if [[ "${CURSOR_DEV_WAIT}" != "0" ]] && [[ "${CURSOR_DEV_WAIT}" != "none" ]] \
  && [[ "${CURSOR_DEV_WAITABLE:-0}" == "1" ]] && [[ -n "${LAUNCH_PID:-}" ]]; then
  (
    _start=$(date +%s)
    wait "$LAUNCH_PID" 2>/dev/null || true
    _elapsed=$(( $(date +%s) - _start ))
    _poll="${CURSOR_DEV_POLL_SEC:-1}"
    if cursor_dev_is_msysish && [[ "${_elapsed}" -lt 3 ]] && command -v tasklist >/dev/null 2>&1; then
      if tasklist //FI "IMAGENAME eq Cursor.exe" 2>/dev/null | grep -qi 'Cursor.exe'; then
        printf '[cursor-dev] Launcher exited quickly; waiting for Cursor.exe to finish (all Cursor windows).\n' >&2
        printf '  Multiple Cursor windows? Set CURSOR_DEV_WAIT=0 and use: docker stop %s\n' "$NAME" >&2
        while tasklist //FI "IMAGENAME eq Cursor.exe" 2>/dev/null | grep -qi 'Cursor.exe'; do
          sleep "$_poll"
        done
      fi
    fi
    cursor_dev_docker_no_pathconv stop "$NAME" >/dev/null 2>&1 || true
  ) &
  CURSOR_BG_WAIT_PID=$!
fi

printf '\n[cursor-dev] Waiting on container (docker wait). When it stops, this workflow exits.\n' >&2
docker wait "$NAME" >/dev/null || true

trap - INT TERM
[[ -n "${CURSOR_BG_WAIT_PID:-}" ]] && kill "$CURSOR_BG_WAIT_PID" 2>/dev/null || true
wait "$CURSOR_BG_WAIT_PID" 2>/dev/null || true
cleanup_session

printf '[cursor-dev] Session ended.\n' >&2
cursor_dev_footer
exit 0
