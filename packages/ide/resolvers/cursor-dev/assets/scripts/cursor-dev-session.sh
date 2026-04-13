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
STATE_ROOT="$(cursor_dev_state_root)"
GLOBAL_STATE_ROOT="$(cursor_dev_global_state_root)"
if [[ "${DOCKPIPE_LAUNCH_MODE:-}" == "gui" ]] || [[ "${DOCKPIPE_LAUNCH_MODE:-}" == "GUI" ]]; then
  printf '[cursor-dev] DOCKPIPE_LAUNCH_MODE=gui — GUI launch (non-server); dockpipe waits on this script until the session ends (Ctrl+C or docker stop <name>).\n' >&2
fi
printf '[dockpipe] AI agent + MCP quickstart (read in Cursor): %s/AGENT-MCP.md\n' "$STATE_ROOT" >&2
if ! cursor_dev_require_docker_for_session; then
  exit 1
fi

IMAGE="${CURSOR_DEV_SESSION_IMAGE:-dockpipe-base-dev:latest}"
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  if docker image inspect dockpipe-base-dev:latest >/dev/null 2>&1; then
    IMAGE=dockpipe-base-dev:latest
  elif docker image inspect dockpipe-base-dev >/dev/null 2>&1; then
    IMAGE=dockpipe-base-dev
  else
    printf '[dockpipe] Image %s not found.\n' "$IMAGE" >&2
    printf '  Set CURSOR_DEV_SESSION_IMAGE to an available image, or build/pull dockpipe-base-dev first.\n' >&2
    exit 1
  fi
fi

# Git Bash / MSYS maps "/work" to C:/Program Files/Git/work when invoking docker.exe. The "//work"
# trick is not reliable on all setups; MSYS2_ARG_CONV_EXCL=* disables path conversion for the whole
# docker invocation (see also vscode-code-server.sh comments).
cursor_dev_docker_no_pathconv() {
  cursor_dev_docker "$@"
}

NAME="${CURSOR_DEV_CONTAINER_NAME:-dockpipe-cursor-dev-${RANDOM}${RANDOM}}"

SESSION_IDLE="${SCRIPT_DIR}/session-idle.sh"
COMMON_SH="${SCRIPT_DIR}/cursor-dev-common.sh"
if [[ ! -f "$SESSION_IDLE" ]]; then
  printf '[dockpipe] missing session script: %s\n' "$SESSION_IDLE" >&2
  exit 1
fi
if [[ ! -f "$COMMON_SH" ]]; then
  printf '[dockpipe] missing shared script: %s\n' "$COMMON_SH" >&2
  exit 1
fi

# So the container can write remote_active before first poll; same paths as cleanup markers.
mkdir -p "${GLOBAL_STATE_ROOT}/cleanup" "$STATE_ROOT"
ACTIVE_SESSION_FILE="${STATE_ROOT}/active-session.env"

cursor_dev_is_running_container() {
  local name="${1:-}"
  [[ -n "$name" ]] || return 1
  local st
  st=$(cursor_dev_docker inspect -f '{{.State.Status}}' "$name" 2>/dev/null) || return 1
  [[ "$st" == "running" ]]
}

cursor_dev_existing_active_session() {
  [[ -f "$ACTIVE_SESSION_FILE" ]] || return 1
  local active_pid="" active_name="" active_workdir=""
  # shellcheck disable=SC1090
  source "$ACTIVE_SESSION_FILE" 2>/dev/null || return 1
  active_pid="${CURSOR_DEV_ACTIVE_PID:-}"
  active_name="${CURSOR_DEV_ACTIVE_CONTAINER:-}"
  active_workdir="${CURSOR_DEV_ACTIVE_WORKDIR:-}"
  if [[ -n "$active_pid" ]] && kill -0 "$active_pid" 2>/dev/null && cursor_dev_is_running_container "$active_name"; then
    printf '[cursor-dev] An active session is already registered for this workdir.\n' >&2
    printf '  Workdir:  %s\n' "${active_workdir:-$W}" >&2
    printf '  PID:      %s\n' "$active_pid" >&2
    printf '  Container:%s%s\n' "${active_name:+ }" "${active_name:- <unknown>}" >&2
    printf '  Reuse the existing session or stop it first: docker stop %s\n' "${active_name:-<name>}" >&2
    return 0
  fi
  rm -f "$ACTIVE_SESSION_FILE"
  return 1
}

if cursor_dev_existing_active_session; then
  exit 0
fi

run_args=(
  run -d --rm --name "$NAME" --init --hostname dockpipe
  -v "${W}:/work"
  -w /work
  -v "${SESSION_IDLE}:/dockpipe-session-idle.sh:ro"
  -v "${COMMON_SH}:/dockpipe-cursor-dev-common.sh:ro"
  -e "HOME=/work/bin/.dockpipe/packages/cursor-dev/home"
  -e "DOCKPIPE_STATE_DIR=/work/bin/.dockpipe"
  -e "DOCKPIPE_PACKAGE_STATE_DIR=/work/bin/.dockpipe/packages/cursor-dev"
  -e "DOCKPIPE_CONTAINER_WORKDIR=/work"
  -e "W=/work"
  -e "IS_SANDBOX=1"
  -e "XDG_CACHE_HOME=/work/bin/.dockpipe/packages/cursor-dev/xdg-cache"
  -e "XDG_CONFIG_HOME=/work/bin/.dockpipe/packages/cursor-dev/xdg-config"
  -e "XDG_DATA_HOME=/work/bin/.dockpipe/packages/cursor-dev/xdg-data"
  -e "GOCACHE=/work/bin/.dockpipe/packages/cursor-dev/gocache"
  -e "CURSOR_DEV_SESSION_POLL_SEC=${CURSOR_DEV_SESSION_POLL_SEC:-2}"
  -e "CURSOR_DEV_CONTAINER_MONITOR=${CURSOR_DEV_CONTAINER_MONITOR:-1}"
  -e "CURSOR_DEV_REMOTE_FS_SIGNAL=${CURSOR_DEV_REMOTE_FS_SIGNAL:-1}"
  -e "CURSOR_DEV_REMOTE_FS_QUIET_SEC=${CURSOR_DEV_REMOTE_FS_QUIET_SEC:-90}"
  -e "CURSOR_DEV_SESSION_LOG_HEARTBEAT_SEC=${CURSOR_DEV_SESSION_LOG_HEARTBEAT_SEC:-0}"
)
case "${OSTYPE:-}" in
  linux-gnu*|darwin*)
    run_args+=(-u "$(id -u):$(id -g)")
    ;;
esac
# session-idle.sh: bootstrap, optional monitor (stdout + bin/.dockpipe/packages/cursor-dev/remote_active), sleep infinity.
run_args+=(--entrypoint /bin/bash "$IMAGE" /dockpipe-session-idle.sh)

cursor_dev_docker_no_pathconv "${run_args[@]}" >/dev/null
cat >"$ACTIVE_SESSION_FILE" <<EOF
CURSOR_DEV_ACTIVE_PID=$$
CURSOR_DEV_ACTIVE_CONTAINER=$NAME
CURSOR_DEV_ACTIVE_WORKDIR=$W
EOF
# Core host cleanup: RunHostScript defer stops any name listed here (and legacy session_container).
printf '%s' "$NAME" > "${GLOBAL_STATE_ROOT}/cleanup/docker-session"
printf '%s' "$NAME" > "${STATE_ROOT}/session_container"
# Legacy path kept for older docs/tools; defer reads .dockpipe/cleanup/docker-* first.
if [[ -n "${DOCKPIPE_RUN_ID:-}" ]]; then
  mkdir -p "${GLOBAL_STATE_ROOT}/runs"
  printf '%s' "$NAME" > "${GLOBAL_STATE_ROOT}/runs/${DOCKPIPE_RUN_ID}.container"
fi

# Register cleanup as soon as the session container exists so set -e / early exit still tears down
# (e.g. cursor_dev_print_instructions or later steps failing). EXIT covers normal end + signals.
cleanup_session() {
  [[ -n "${NAME:-}" ]] || return 0
  [[ -n "${CURSOR_BG_WAIT_PID:-}" ]] && kill "$CURSOR_BG_WAIT_PID" 2>/dev/null || true
  cursor_dev_docker_no_pathconv stop "$NAME" >/dev/null 2>&1 || true
  if [[ -f "$ACTIVE_SESSION_FILE" ]]; then
    # Remove only the record for this session; a later launch may have replaced it.
    # shellcheck disable=SC1090
    source "$ACTIVE_SESSION_FILE" 2>/dev/null || true
    if [[ "${CURSOR_DEV_ACTIVE_PID:-}" == "$$" ]] && [[ "${CURSOR_DEV_ACTIVE_CONTAINER:-}" == "${NAME:-}" ]]; then
      rm -f "$ACTIVE_SESSION_FILE"
    fi
  fi
  rm -f "${GLOBAL_STATE_ROOT}/cleanup/docker-session"
  rm -f "${STATE_ROOT}/session_container"
  if [[ -n "${DOCKPIPE_RUN_ID:-}" ]]; then
    rm -f "${GLOBAL_STATE_ROOT}/runs/${DOCKPIPE_RUN_ID}.container"
  fi
}
trap cleanup_session INT TERM HUP EXIT

export CURSOR_DEV_CONTAINER_NAME="$NAME"
export CURSOR_DEV_FOLDER_URI=""
# Attach Cursor to this running container and open /work (same authority Dev Containers uses in the UI).
if [[ "${CURSOR_DEV_REMOTE_URI:-1}" != "0" ]]; then
  if cursor_dev_wait_container_running "$NAME" 120; then
    if uri=$(cursor_dev_dev_container_folder_uri "$NAME" "/work"); then
      export CURSOR_DEV_FOLDER_URI="$uri"
      printf '[cursor-dev] Dev container URI ready — Cursor will attach and open /work.\n' >&2
    fi
  else
    printf '[dockpipe] Warning: session container did not become running in time; opening host folder instead.\n' >&2
  fi
fi

printf '\n[cursor-dev] Session container is running (same idea as vscode: dockpipe waits on Docker).\n' >&2
printf '  Name:     %s\n' "$NAME" >&2
printf '  Project:  %s  →  /work in the container\n' "$W" >&2
printf '  Stop:     docker stop %s   or press Ctrl+C here (stops the container).\n' "$NAME" >&2

cursor_dev_print_instructions

CURSOR_LAUNCHED=0
if try_launch_cursor; then
  CURSOR_LAUNCHED=1
else
  printf '\n[cursor-dev] Cursor CLI not found — open the folder above manually in Cursor.\n' >&2
fi

printf '[dockpipe-ready] cursor-dev\n' >&2

# When Cursor exits, stop the session container (CURSOR_DEV_WAIT=1, default). The script waits for the
# GUI process (not the launcher PID): on Linux/macOS the `cursor` CLI often returns before Electron starts.
# Set CURSOR_DEV_WAIT=0 to keep the container until Ctrl+C or docker stop. See README.
CURSOR_DEV_WAIT="${CURSOR_DEV_WAIT:-1}"
CURSOR_BG_WAIT_PID=""

# Always start the shutdown watcher when CURSOR_DEV_WAIT=1 — even if try_launch_cursor failed (user may attach
# manually). Otherwise the container never stops when you close the remote window or quit Cursor.
if [[ "${CURSOR_DEV_WAIT}" != "0" ]] && [[ "${CURSOR_DEV_WAIT}" != "none" ]]; then
  if [[ "${CURSOR_LAUNCHED}" != "1" ]]; then
    printf '[cursor-dev] Cursor was not auto-launched — still monitoring for session end (attach manually if needed).\n' >&2
  fi
  (
    if [[ -n "${LAUNCH_PID:-}" ]] && [[ "${CURSOR_DEV_WAITABLE:-0}" == "1" ]]; then
      wait "$LAUNCH_PID" 2>/dev/null || true
    fi
    if cursor_dev_wait_for_cursor_gui_exit "$NAME"; then
      cursor_dev_docker_no_pathconv stop "$NAME" >/dev/null 2>&1 || true
    fi
  ) &
  CURSOR_BG_WAIT_PID=$!
fi

printf '\n[cursor-dev] Waiting on container (docker wait). When it stops, this workflow exits.\n' >&2
docker wait "$NAME" >/dev/null || true

trap - INT TERM
[[ -n "${CURSOR_BG_WAIT_PID:-}" ]] && kill "$CURSOR_BG_WAIT_PID" 2>/dev/null || true
wait "$CURSOR_BG_WAIT_PID" 2>/dev/null || true

printf '[cursor-dev] Session ended.\n' >&2
cursor_dev_footer
exit 0
