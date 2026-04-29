#!/usr/bin/env bash
set -euo pipefail

eval "$(dockpipe sdk)"
dockpipe_sdk init-script

report_launch_failure() {
  local exit_code="$1"
  local line_no="$2"
  local command="$3"
  printf 'pipeon-dev-stack: launch.sh failed at line %s while running: %s (exit %s)\n' \
    "$line_no" "$command" "$exit_code" >&2
}

trap 'report_launch_failure "$?" "${LINENO}" "${BASH_COMMAND}"' ERR
# shellcheck source=/dev/null
source "$SCRIPT_DIR/common.sh"

WORKDIR="$(pipeon_stack_workdir)"
PROJECT_DIR="$(pipeon_stack_repo_root)"
COMPOSE_FILE="$(pipeon_stack_compose_file)"
COMPOSE_PROJECT="$(pipeon_stack_compose_project)"
LEGACY_COMPOSE_PROJECT="$(pipeon_stack_legacy_compose_project)"
CODE_SERVER_CONTAINER_NAME="$(pipeon_stack_code_server_name)"
LEGACY_CODE_SERVER_CONTAINER_NAME="$(pipeon_stack_legacy_code_server_name)"
CODE_SERVER_URL="$(pipeon_stack_code_server_url)"
MCP_PORT="$(pipeon_stack_mcp_port)"
MCP_URL="$(pipeon_stack_mcp_url)"
RUNTIME_ENV="$(pipeon_stack_runtime_env)"
AUTODOWN="${PIPEON_DEV_STACK_AUTODOWN:-1}"
BUILD_MODE="${PIPEON_DEV_STACK_BUILD:-auto}"
MODEL_NAME="${PIPEON_OLLAMA_MODEL:-${DOCKPIPE_OLLAMA_MODEL:-llama3.2}}"
PIPEON_DESKTOP_BIN="${PIPEON_DESKTOP_BIN:-$(pipeon_stack_desktop_bin)}"
PIPEON_DESKTOP_SCRIPT="$SCRIPT_DIR/desktop.sh"

resolve_tool_bin() {
  local configured="${1:-}"
  local command_name="$2"
  if [[ -n "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi
  if command -v "$command_name" >/dev/null 2>&1; then
    command -v "$command_name"
    return 0
  fi
  return 1
}

resolve_repo_tool_bin() {
  local configured="${1:-}"
  local command_name="$2"
  shift 2
  local candidate

  if [[ -n "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi

  for candidate in "$@"; do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done

  resolve_tool_bin "" "$command_name"
}

resolve_pipeon_bin() {
  local configured="${1:-}"
  local candidate
  if [[ -n "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi

  for candidate in \
    "$WORKDIR/packages/pipeon/resolvers/pipeon/bin/pipeon" \
    "$PROJECT_DIR/packages/pipeon/resolvers/pipeon/bin/pipeon"
  do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done

  resolve_tool_bin "" pipeon
}

compose_cmd() {
  local args=()
  mapfile -t args < <(pipeon_stack_compose_base_args)
  docker compose "${args[@]}" "$@"
}

wait_for_ollama_ready() {
  local attempts="${1:-60}"
  local i
  for ((i = 0; i < attempts; i++)); do
    if compose_cmd exec -T ollama ollama list >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

wait_for_mcp_ready() {
  local attempts="${2:-40}"
  local i code
  for ((i = 0; i < attempts; i++)); do
    code="$(
      curl -sS -o /dev/null -w '%{http_code}' \
        "$MCP_URL" 2>/dev/null || true
    )"
    case "$code" in
      200|204|400|401|405)
        return 0
        ;;
    esac
    sleep 0.25
  done
  return 1
}

DOCKPIPE_BIN="$(resolve_repo_tool_bin "${DOCKPIPE_BIN:-}" dockpipe \
  "$WORKDIR/src/bin/dockpipe" \
  "$PROJECT_DIR/src/bin/dockpipe")"
DORKPIPE_BIN="$(resolve_repo_tool_bin "${DORKPIPE_BIN:-}" dorkpipe \
  "$WORKDIR/packages/dorkpipe/bin/dorkpipe" \
  "$PROJECT_DIR/packages/dorkpipe/bin/dorkpipe")"
MCPD_BIN="$(resolve_repo_tool_bin "${MCPD_BIN:-}" mcpd \
  "$WORKDIR/packages/dorkpipe/bin/mcpd" \
  "$PROJECT_DIR/packages/dorkpipe/bin/mcpd")"
PIPEON_BIN="$(resolve_pipeon_bin "${PIPEON_BIN:-}")"

ensure_pipeon_stack_state_dir
ensure_pipeon_stack_api_key
ensure_pipeon_stack_mcp_tls_material
write_pipeon_stack_runtime_env

cleanup() {
  if [[ "$AUTODOWN" == "1" ]]; then
    docker rm -f "$CODE_SERVER_CONTAINER_NAME" >/dev/null 2>&1 || true
    docker rm -f "$LEGACY_CODE_SERVER_CONTAINER_NAME" >/dev/null 2>&1 || true
    docker ps -aq --filter "label=com.dockpipe.stack.project=$COMPOSE_PROJECT" | xargs -r docker rm -f >/dev/null 2>&1 || true
    docker ps -aq --filter "label=com.dockpipe.stack.project=$LEGACY_COMPOSE_PROJECT" | xargs -r docker rm -f >/dev/null 2>&1 || true
  fi
  if [[ "$AUTODOWN" == "1" ]]; then
    compose_cmd down >/dev/null 2>&1 || true
    docker compose --env-file "$RUNTIME_ENV" -p "$LEGACY_COMPOSE_PROJECT" -f "$COMPOSE_FILE" --project-directory "$PROJECT_DIR" down >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

if ! docker version >/dev/null 2>&1; then
  echo "pipeon-dev-stack: Docker is not reachable" >&2
  exit 1
fi

configure_pipeon_stack_gpu

case "${PIPEON_DEV_STACK_PROMPT_RESULT:-}" in
  gpu-setup)
    echo "pipeon-dev-stack: launch paused before starting services so Docker GPU access can be enabled" >&2
    exit 0
    ;;
  cancelled)
    echo "pipeon-dev-stack: launch cancelled before starting services" >&2
    exit 0
    ;;
esac

case "$BUILD_MODE" in
  always)
    printf '[pipeon-dev-stack] PIPEON_DEV_STACK_BUILD=always no longer rebuilds sibling source trees; supply DOCKPIPE_BIN, DORKPIPE_BIN, MCPD_BIN, and PIPEON_DESKTOP_BIN explicitly if needed.\n' >&2
    ;;
  auto)
    :
    ;;
  never)
    ;;
  *)
    echo "pipeon-dev-stack: PIPEON_DEV_STACK_BUILD must be auto, always, or never (got $BUILD_MODE)" >&2
    exit 1
    ;;
esac

if [[ ! -x "$DOCKPIPE_BIN" || ! -x "$DORKPIPE_BIN" || ! -x "$MCPD_BIN" ]]; then
  echo "pipeon-dev-stack: required binaries are missing after build step" >&2
  echo "  dockpipe: ${DOCKPIPE_BIN:-<unset>}" >&2
  echo "  dorkpipe: ${DORKPIPE_BIN:-<unset>} (set DORKPIPE_BIN or add dorkpipe to PATH)" >&2
  echo "  mcpd:     ${MCPD_BIN:-<unset>} (set MCPD_BIN or add mcpd to PATH)" >&2
  exit 1
fi

if [[ ! -f "$PIPEON_DESKTOP_SCRIPT" ]]; then
  echo "pipeon-dev-stack: missing desktop launcher script at $PIPEON_DESKTOP_SCRIPT" >&2
  exit 1
fi

compose_cmd up -d --remove-orphans

if ! verify_pipeon_stack_ollama_gpu; then
  compose_cmd logs ollama >&2 || true
  exit 1
fi

if ! wait_for_mcp_ready 40; then
  echo "pipeon-dev-stack: isolated DorkPipe MCP boundary did not become reachable at $MCP_URL" >&2
  compose_cmd logs dorkpipe-stack pipeon-mcp-proxy >&2 || true
  exit 1
fi

export CODE_SERVER_WAIT="${CODE_SERVER_WAIT:-1}"
export CODE_SERVER_AUTH="${CODE_SERVER_AUTH:-none}"
export CODE_SERVER_CONTAINER_NAME="$CODE_SERVER_CONTAINER_NAME"
export DORKPIPE_DEV_STACK_PROJECT="$COMPOSE_PROJECT"
export CODE_SERVER_URL="$CODE_SERVER_URL"
export PIPEON_WINDOW_TITLE="${PIPEON_WINDOW_TITLE:-Pipeon}"
export DOCKPIPE_PIPEON="${DOCKPIPE_PIPEON:-1}"
export DOCKPIPE_PIPEON_ALLOW_PRERELEASE="${DOCKPIPE_PIPEON_ALLOW_PRERELEASE:-1}"
export PIPEON_OLLAMA_MODEL="${PIPEON_OLLAMA_MODEL:-$MODEL_NAME}"
export MCP_HTTP_URL="$MCP_URL"
export MCP_HTTP_CONTAINER_URL="$(pipeon_stack_mcp_container_url)"
export PIPEON_DESKTOP_BIN

if [[ "${CODE_SERVER_IMAGE:-dockpipe-code-server:latest}" == "dockpipe-code-server:latest" ]] \
  && ! docker image inspect dockpipe-code-server:latest >/dev/null 2>&1; then
  export CODE_SERVER_IMAGE="codercom/code-server:latest"
  printf '[pipeon-dev-stack] dockpipe-code-server:latest was not available; falling back to %s (Pipeon branding/extension may be reduced)\n' "$CODE_SERVER_IMAGE" >&2
fi

if [[ "${PIPEON_DEV_STACK_PULL_MODEL:-1}" == "1" ]]; then
  printf '[pipeon-dev-stack] ensuring Ollama model %s is available...\n' "$MODEL_NAME" >&2
  if ! wait_for_ollama_ready 90; then
    echo "pipeon-dev-stack: Ollama did not become ready in time" >&2
    exit 1
  fi
  compose_cmd exec -T ollama ollama pull "$MODEL_NAME" >&2
fi

if [[ "${PIPEON_DEV_STACK_PIPEON_BUNDLE:-1}" == "1" && -x "$PIPEON_BIN" ]]; then
  if ! (
    cd "$WORKDIR"
    export DOCKPIPE_BIN
    export DOCKPIPE_WORKDIR="$WORKDIR"
    "$PIPEON_BIN" bundle
  ) >&2; then
    echo "pipeon-dev-stack: pipeon bundle failed using $PIPEON_BIN" >&2
    exit 1
  fi
fi

cat >&2 <<EOF
[dockpipe-ready] pipeon-dev-stack
[pipeon-dev-stack] ready
  workdir:      $WORKDIR
  ide:          Pipeon
  ui:           $CODE_SERVER_URL
  mcp:          $MCP_URL
  mcp api key:  $(pipeon_stack_api_key_file)
  dorkpipe:     isolated compose service
  state:        $(pipeon_stack_state_dir)
  control:      isolated compose service dorkpipe-stack
  ollama gpu:   $(pipeon_stack_gpu_mode)
  gpu status:   $(pipeon_stack_gpu_status)
EOF

if ! bash "$PIPEON_DESKTOP_SCRIPT"; then
  desktop_status=$?
  AUTODOWN=0
  printf '[pipeon-dev-stack] Pipeon desktop shell failed to launch (exit %s)\n' "$desktop_status" >&2
  printf '[pipeon-dev-stack] the stack is still running; open Pipeon manually at %s\n' "$CODE_SERVER_URL" >&2
  exit 0
fi
