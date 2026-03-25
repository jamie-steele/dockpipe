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
if [[ "${DOCKPIPE_LAUNCH_MODE:-}" == "gui" ]] || [[ "${DOCKPIPE_LAUNCH_MODE:-}" == "GUI" ]]; then
  printf '[cursor-dev] DOCKPIPE_LAUNCH_MODE=gui — GUI launch (non-server); dockpipe waits on this script until the session ends (Ctrl+C or docker stop <name>).\n' >&2
fi
printf '[dockpipe] AI agent + MCP quickstart (read in Cursor): %s/.dockpipe/cursor-dev/AGENT-MCP.md\n' "$W" >&2
if ! cursor_dev_require_docker_for_session; then
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
mkdir -p "$W/.dockpipe/cleanup" "$W/.dockpipe/cursor-dev"

run_args=(
  run -d --rm --name "$NAME" --init --hostname dockpipe
  -v "${W}:/work"
  -w /work
  -v "${SESSION_IDLE}:/dockpipe-session-idle.sh:ro"
  -v "${COMMON_SH}:/dockpipe-cursor-dev-common.sh:ro"
  -e "HOME=/work"
  -e "DOCKPIPE_CONTAINER_WORKDIR=/work"
  -e "W=/work"
  -e "IS_SANDBOX=1"
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
# session-idle.sh: bootstrap, optional monitor (stdout + .dockpipe/cursor-dev/remote_active), sleep infinity.
run_args+=(--entrypoint /bin/bash "$IMAGE" /dockpipe-session-idle.sh)

cursor_dev_docker_no_pathconv "${run_args[@]}" >/dev/null
# Core host cleanup: RunHostScript defer stops any name listed here (and legacy session_container).
printf '%s' "$NAME" > "$W/.dockpipe/cleanup/docker-session"
printf '%s' "$NAME" > "$W/.dockpipe/cursor-dev/session_container"
# Legacy path kept for older docs/tools; defer reads .dockpipe/cleanup/docker-* first.
if [[ -n "${DOCKPIPE_RUN_ID:-}" ]]; then
  mkdir -p "$W/.dockpipe/runs"
  printf '%s' "$NAME" > "$W/.dockpipe/runs/${DOCKPIPE_RUN_ID}.container"
fi

# Register cleanup as soon as the session container exists so set -e / early exit still tears down
# (e.g. cursor_dev_print_instructions or later steps failing). EXIT covers normal end + signals.
cleanup_session() {
  [[ -n "${NAME:-}" ]] || return 0
  [[ -n "${CURSOR_BG_WAIT_PID:-}" ]] && kill "$CURSOR_BG_WAIT_PID" 2>/dev/null || true
  cursor_dev_docker_no_pathconv stop "$NAME" >/dev/null 2>&1 || true
  rm -f "$W/.dockpipe/cleanup/docker-session"
  rm -f "$W/.dockpipe/cursor-dev/session_container"
  if [[ -n "${DOCKPIPE_RUN_ID:-}" ]]; then
    rm -f "$W/.dockpipe/runs/${DOCKPIPE_RUN_ID}.container"
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
