#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/vscode-common.sh"

vscode_set_workdir
if [[ "${DOCKPIPE_LAUNCH_MODE:-}" == "gui" ]] || [[ "${DOCKPIPE_LAUNCH_MODE:-}" == "GUI" ]]; then
  printf '[vscode] DOCKPIPE_LAUNCH_MODE=gui — desktop VS Code attaches to a session container.\n' >&2
fi
if ! vscode_require_docker_for_session; then
  exit 1
fi

vscode_repo_root() {
  local r=""
  if [[ -n "${DOCKPIPE_REPO_ROOT:-}" ]]; then
    r="$(cd "$DOCKPIPE_REPO_ROOT" 2>/dev/null && pwd || true)"
  fi
  if [[ -z "$r" ]] || [[ ! -f "$r/templates/core/assets/images/base-dev/Dockerfile" ]]; then
    r="$(cd "$SCRIPT_DIR/../../../../../../" 2>/dev/null && pwd || true)"
  fi
  printf '%s' "$r"
}
REPO_ROOT="$(vscode_repo_root)"

IMAGE="${VSCODE_SESSION_IMAGE:-dockpipe-base-dev:latest}"
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  if docker image inspect dockpipe-base-dev:latest >/dev/null 2>&1; then
    IMAGE=dockpipe-base-dev:latest
  elif docker image inspect dockpipe-base-dev >/dev/null 2>&1; then
    IMAGE=dockpipe-base-dev
  elif [[ -n "$REPO_ROOT" ]] && [[ -f "$REPO_ROOT/templates/core/assets/images/base-dev/Dockerfile" ]]; then
    printf '[dockpipe] Building dockpipe-base-dev image from the local DockPipe checkout…\n' >&2
    docker build -q -t dockpipe-base-dev -f "$REPO_ROOT/templates/core/assets/images/base-dev/Dockerfile" "$REPO_ROOT"
    IMAGE=dockpipe-base-dev:latest
  else
    printf '[dockpipe] Image dockpipe-base-dev not found and could not build.\n' >&2
    exit 1
  fi
fi

NAME="${VSCODE_CONTAINER_NAME:-dockpipe-vscode-dev-${RANDOM}${RANDOM}}"
SESSION_IDLE="${SCRIPT_DIR}/session-idle.sh"
COMMON_SH="${SCRIPT_DIR}/vscode-common.sh"
STATE_ROOT="$(vscode_state_root)"
GLOBAL_STATE_ROOT="$(vscode_global_state_root)"
STATE_HOME="$(vscode_home_dir)"
STATE_CACHE="${STATE_ROOT}/xdg-cache"
STATE_CONFIG="${STATE_ROOT}/xdg-config"
STATE_DATA="${STATE_ROOT}/xdg-data"
STATE_DOTNET="${STATE_ROOT}/dotnet"
STATE_GOCACHE="${STATE_ROOT}/gocache"
STATE_SERVER="$(vscode_remote_server_dir)"
mkdir -p \
  "${GLOBAL_STATE_ROOT}/cleanup" \
  "$STATE_ROOT" \
  "$STATE_HOME" \
  "$STATE_CACHE" \
  "$STATE_CONFIG" \
  "$STATE_DATA" \
  "$STATE_DOTNET" \
  "$STATE_GOCACHE" \
  "$STATE_SERVER"
ACTIVE_SESSION_FILE="${STATE_ROOT}/active-session.env"

vscode_migrate_repo_state_dir() {
  local name="$1"
  local src="${W}/${name}"
  local dst="${STATE_HOME}/${name}"
  [[ -e "$src" ]] || return 0
  [[ -e "$dst" ]] && return 0
  printf '[vscode] Moving repo-local %s into %s\n' "$name" "$STATE_ROOT" >&2
  mv "$src" "$dst"
}

vscode_migrate_repo_gitconfig() {
  local src="${W}/.gitconfig"
  local dst="${STATE_HOME}/.gitconfig"
  [[ -f "$src" ]] || return 0
  [[ -e "$dst" ]] && return 0
  if grep -q 'vscode-remote-containers-.*git-credential-helper' "$src" 2>/dev/null; then
    printf '[vscode] Moving repo-local .gitconfig Dev Containers helper into %s\n' "$STATE_ROOT" >&2
    mv "$src" "$dst"
  fi
}

vscode_migrate_repo_state_dir ".cache"
vscode_migrate_repo_state_dir ".copilot"
vscode_migrate_repo_state_dir ".dotnet"
vscode_migrate_repo_state_dir ".vscode-server"
vscode_migrate_repo_state_dir ".gocache"
vscode_migrate_repo_gitconfig

vscode_is_running_container() {
  local name="${1:-}"
  [[ -n "$name" ]] || return 1
  local st
  st=$(vscode_docker inspect -f '{{.State.Status}}' "$name" 2>/dev/null) || return 1
  [[ "$st" == "running" ]]
}

if [[ -f "$ACTIVE_SESSION_FILE" ]]; then
  # shellcheck disable=SC1090
  source "$ACTIVE_SESSION_FILE" 2>/dev/null || true
  if [[ -n "${VSCODE_ACTIVE_PID:-}" ]] && kill -0 "${VSCODE_ACTIVE_PID}" 2>/dev/null && vscode_is_running_container "${VSCODE_ACTIVE_CONTAINER:-}"; then
    printf '[vscode] An active session already exists for this workdir.\n' >&2
    printf '  Container: %s\n' "${VSCODE_ACTIVE_CONTAINER:-<unknown>}" >&2
    exit 0
  fi
  rm -f "$ACTIVE_SESSION_FILE"
fi

run_args=(
  run -d --rm --name "$NAME" --init --hostname dockpipe
  -v "${W}:/work"
  -w /work
  -v "${SESSION_IDLE}:/dockpipe-session-idle.sh:ro"
  -v "${COMMON_SH}:/dockpipe-vscode-common.sh:ro"
  -e "HOME=/work/bin/.dockpipe/packages/vscode/home"
  -e "DOCKPIPE_STATE_DIR=/work/bin/.dockpipe"
  -e "DOCKPIPE_PACKAGE_STATE_DIR=/work/bin/.dockpipe/packages/vscode"
  -e "DOCKPIPE_CONTAINER_WORKDIR=/work"
  -e "W=/work"
  -e "IS_SANDBOX=1"
  -e "GIT_CONFIG_GLOBAL=/work/bin/.dockpipe/packages/vscode/home/.gitconfig"
  -e "XDG_CACHE_HOME=/work/bin/.dockpipe/packages/vscode/xdg-cache"
  -e "XDG_CONFIG_HOME=/work/bin/.dockpipe/packages/vscode/xdg-config"
  -e "XDG_DATA_HOME=/work/bin/.dockpipe/packages/vscode/xdg-data"
  -e "DOTNET_CLI_HOME=/work/bin/.dockpipe/packages/vscode/dotnet"
  -e "GOCACHE=/work/bin/.dockpipe/packages/vscode/gocache"
  -e "VSCODE_SESSION_POLL_SEC=${VSCODE_SESSION_POLL_SEC:-2}"
  -e "VSCODE_CONTAINER_MONITOR=${VSCODE_CONTAINER_MONITOR:-1}"
  -e "VSCODE_REMOTE_FS_SIGNAL=${VSCODE_REMOTE_FS_SIGNAL:-1}"
  -e "VSCODE_REMOTE_FS_QUIET_SEC=${VSCODE_REMOTE_FS_QUIET_SEC:-90}"
)
case "${OSTYPE:-}" in
  linux-gnu*|darwin*)
    run_args+=(-u "$(id -u):$(id -g)")
    ;;
esac
run_args+=(--entrypoint /bin/bash "$IMAGE" /dockpipe-session-idle.sh)

vscode_docker "${run_args[@]}" >/dev/null
cat >"$ACTIVE_SESSION_FILE" <<EOF
VSCODE_ACTIVE_PID=$$
VSCODE_ACTIVE_CONTAINER=$NAME
EOF
printf '%s' "$NAME" > "$W/bin/.dockpipe/cleanup/docker-session"
printf '%s' "$NAME" > "${STATE_ROOT}/session_container"
if [[ -n "${DOCKPIPE_RUN_ID:-}" ]]; then
  mkdir -p "$W/bin/.dockpipe/runs"
  printf '%s' "$NAME" > "$W/bin/.dockpipe/runs/${DOCKPIPE_RUN_ID}.container"
fi

cleanup_session() {
  [[ -n "${NAME:-}" ]] || return 0
  [[ -n "${VSCODE_BG_WAIT_PID:-}" ]] && kill "$VSCODE_BG_WAIT_PID" 2>/dev/null || true
  vscode_docker stop "$NAME" >/dev/null 2>&1 || true
  if [[ -f "$ACTIVE_SESSION_FILE" ]]; then
    # shellcheck disable=SC1090
    source "$ACTIVE_SESSION_FILE" 2>/dev/null || true
    if [[ "${VSCODE_ACTIVE_PID:-}" == "$$" ]] && [[ "${VSCODE_ACTIVE_CONTAINER:-}" == "${NAME:-}" ]]; then
      rm -f "$ACTIVE_SESSION_FILE"
    fi
  fi
  rm -f "${GLOBAL_STATE_ROOT}/cleanup/docker-session" "${STATE_ROOT}/session_container"
  if [[ -n "${DOCKPIPE_RUN_ID:-}" ]]; then
    rm -f "$W/bin/.dockpipe/runs/${DOCKPIPE_RUN_ID}.container"
  fi
}
trap cleanup_session INT TERM HUP EXIT

if ! vscode_wait_container_running "$NAME" 120; then
  printf '[dockpipe] Session container did not become ready in time.\n' >&2
  exit 1
fi

uri="$(vscode_attached_container_folder_uri "$NAME" "/work")" || {
  printf '[dockpipe] Failed to build VS Code attached-container URI.\n' >&2
  exit 1
}

printf '\n[vscode] Session container is running.\n' >&2
printf '  Name:    %s\n' "$NAME" >&2
printf '  Project: %s -> /work in the container\n' "$W" >&2
printf '  State:   %s\n' "$STATE_ROOT" >&2

if ! exe=$(vscode_resolve_exe); then
  printf '[vscode] VS Code CLI not found — install VS Code or set VSCODE_CMD.\n' >&2
  exit 1
fi
if ! launch_vscode_folder_uri "$exe" "$uri"; then
  printf '[vscode] Failed to launch VS Code.\n' >&2
  exit 1
fi

printf '[dockpipe-ready] vscode\n' >&2

VSCODE_WAIT="${VSCODE_WAIT:-1}"
if [[ "${VSCODE_WAIT}" != "0" ]] && [[ "${VSCODE_WAIT}" != "none" ]]; then
  (
    vscode_wait_for_session_end "$NAME"
    vscode_docker stop "$NAME" >/dev/null 2>&1 || true
  ) &
  VSCODE_BG_WAIT_PID=$!
fi

printf '\n[vscode] Waiting on container (docker wait). When it stops, this workflow exits.\n' >&2
docker wait "$NAME" >/dev/null || true

trap - INT TERM
[[ -n "${VSCODE_BG_WAIT_PID:-}" ]] && kill "$VSCODE_BG_WAIT_PID" 2>/dev/null || true
wait "$VSCODE_BG_WAIT_PID" 2>/dev/null || true

printf '[vscode] Session ended.\n' >&2
exit 0
