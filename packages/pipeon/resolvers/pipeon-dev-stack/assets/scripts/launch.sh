#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/common.sh"

WORKDIR="$(pipeon_stack_workdir)"
REPO_ROOT="$(pipeon_stack_repo_root)"
COMPOSE_FILE="$(pipeon_stack_compose_file)"
COMPOSE_PROJECT="$(pipeon_stack_compose_project)"
CODE_SERVER_CONTAINER_NAME="$(pipeon_stack_code_server_name)"
CODE_SERVER_URL="$(pipeon_stack_code_server_url)"
MCP_PORT="$(pipeon_stack_mcp_port)"
MCP_URL="$(pipeon_stack_mcp_url)"
PID_FILE="$(pipeon_stack_pid_file)"
LOG_FILE="$(pipeon_stack_log_file)"
AUTODOWN="${PIPEON_DEV_STACK_AUTODOWN:-1}"
BUILD_MODE="${PIPEON_DEV_STACK_BUILD:-auto}"
MODEL_NAME="${PIPEON_OLLAMA_MODEL:-${DOCKPIPE_OLLAMA_MODEL:-llama3.2}}"
PIPEON_DESKTOP_BIN="${PIPEON_DESKTOP_BIN:-$(pipeon_stack_desktop_bin)}"
PIPEON_DESKTOP_SCRIPT="$SCRIPT_DIR/desktop.sh"
STACK_STARTED_BY_ME=0
MCP_STARTED_BY_ME=0

go_build_in_dir() {
  local dir="$1"
  shift
  (
    cd "$dir"
    env GOCACHE=/tmp/dockpipe-go-build-cache go build "$@"
  )
}

resolve_tool_bin() {
  local configured="${1:-}"
  local command_name="$2"
  local repo_fallback="$3"
  if [[ -n "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi
  if command -v "$command_name" >/dev/null 2>&1; then
    command -v "$command_name"
    return 0
  fi
  printf '%s\n' "$repo_fallback"
}

try_rebuild_or_keep_existing() {
  local target="$1"
  local label="$2"
  shift 2
  if "$@"; then
    return 0
  fi
  if [[ -x "$target" ]]; then
    printf '[pipeon-dev-stack] warning: could not rebuild %s; using existing binary at %s\n' "$label" "$target" >&2
    return 0
  fi
  printf '[pipeon-dev-stack] error: rebuild failed and no existing %s binary is available at %s\n' "$label" "$target" >&2
  return 1
}

paths_newer_than() {
  local target="$1"
  shift
  if [[ ! -e "$target" ]]; then
    return 0
  fi
  local path
  for path in "$@"; do
    [[ -e "$path" ]] || continue
    if [[ -d "$path" ]]; then
      if find "$path" -type f -newer "$target" -print -quit 2>/dev/null | grep -q .; then
        return 0
      fi
    elif [[ "$path" -nt "$target" ]]; then
      return 0
    fi
  done
  return 1
}

code_server_image_needs_rebuild() {
  local stamp
  stamp="$(pipeon_stack_image_stamp_file)"
  paths_newer_than "$stamp" \
    "$REPO_ROOT/packages/pipeon/resolvers/pipeon/vscode-extension/Dockerfile.code-server" \
    "$REPO_ROOT/packages/pipeon/resolvers/pipeon/vscode-extension/code-server-user-settings.json" \
    "$REPO_ROOT/packages/pipeon/resolvers/pipeon/vscode-extension/extension.js" \
    "$REPO_ROOT/packages/pipeon/resolvers/pipeon/vscode-extension/package.json" \
    "$REPO_ROOT/packages/dockpipe-language-support" \
    "$REPO_ROOT/packages/pipeon/resolvers/pipeon/vscode-extension/images"
}

build_code_server_image_if_needed() {
  local stamp
  stamp="$(pipeon_stack_image_stamp_file)"
  ensure_pipeon_stack_state_dir
  if ! docker image inspect dockpipe-code-server:latest >/dev/null 2>&1 || code_server_image_needs_rebuild; then
    printf '[pipeon-dev-stack] rebuilding dockpipe-code-server:latest for fresh extension assets...\n' >&2
    make build-code-server-image >&2
    date -Iseconds > "$stamp"
  fi
}

wait_for_ollama_ready() {
  local attempts="${1:-60}"
  local i
  for ((i = 0; i < attempts; i++)); do
    if docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --project-directory "$REPO_ROOT" exec -T ollama ollama list >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

wait_for_mcp_ready() {
  local api_key="$1"
  local attempts="${2:-40}"
  local i code
  for ((i = 0; i < attempts; i++)); do
    code="$(
      curl -sS -o /dev/null -w '%{http_code}' \
        -H "Authorization: Bearer $api_key" \
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

DOCKPIPE_BIN="$(resolve_tool_bin "${DOCKPIPE_BIN:-}" dockpipe "$REPO_ROOT/src/bin/dockpipe")"
DORKPIPE_BIN="$(resolve_tool_bin "${DORKPIPE_BIN:-}" dorkpipe "$REPO_ROOT/packages/dorkpipe/bin/dorkpipe")"
MCPD_BIN="$(resolve_tool_bin "${MCPD_BIN:-}" mcpd "$REPO_ROOT/packages/dorkpipe-mcp/bin/mcpd")"
PIPEON_BIN="$(resolve_tool_bin "${PIPEON_BIN:-}" pipeon "$REPO_ROOT/packages/pipeon/resolvers/pipeon/bin/pipeon")"

ensure_pipeon_stack_state_dir
ensure_pipeon_stack_api_key

cleanup() {
  if [[ "$AUTODOWN" == "1" ]]; then
    docker rm -f "$CODE_SERVER_CONTAINER_NAME" >/dev/null 2>&1 || true
  fi
  if [[ "$MCP_STARTED_BY_ME" == "1" && "$AUTODOWN" == "1" ]]; then
    pipeon_stack_stop_mcpd
  fi
  if [[ "$AUTODOWN" == "1" ]]; then
    docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --project-directory "$REPO_ROOT" down >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

if ! docker version >/dev/null 2>&1; then
  echo "pipeon-dev-stack: Docker is not reachable" >&2
  exit 1
fi

case "$BUILD_MODE" in
  always)
    try_rebuild_or_keep_existing "$DOCKPIPE_BIN" dockpipe env GOCACHE=/tmp/dockpipe-go-build-cache make build
    try_rebuild_or_keep_existing "$DORKPIPE_BIN" dorkpipe \
      go_build_in_dir "$REPO_ROOT/packages/dorkpipe/lib" -trimpath -ldflags "-s -w -X main.Version=$(tr -d ' \t\r\n' < "$REPO_ROOT/VERSION")" -o ../bin/dorkpipe ./cmd/dorkpipe
    try_rebuild_or_keep_existing "$MCPD_BIN" mcpd \
      go_build_in_dir "$REPO_ROOT/packages/dorkpipe-mcp" -trimpath -ldflags "-s -w -X main.Version=$(tr -d ' \t\r\n' < "$REPO_ROOT/VERSION")" -o bin/mcpd ./cmd/mcpd
    make build-pipeon-desktop
    ;;
  auto)
    if [[ ! -x "$DOCKPIPE_BIN" ]] || paths_newer_than "$DOCKPIPE_BIN" "$REPO_ROOT/src" "$REPO_ROOT/src/Makefile" "$REPO_ROOT/VERSION"; then
      try_rebuild_or_keep_existing "$DOCKPIPE_BIN" dockpipe env GOCACHE=/tmp/dockpipe-go-build-cache make build
    fi
    if [[ ! -x "$DORKPIPE_BIN" ]] || paths_newer_than "$DORKPIPE_BIN" "$REPO_ROOT/packages/dorkpipe/lib" "$REPO_ROOT/VERSION"; then
      try_rebuild_or_keep_existing "$DORKPIPE_BIN" dorkpipe \
        go_build_in_dir "$REPO_ROOT/packages/dorkpipe/lib" -trimpath -ldflags "-s -w -X main.Version=$(tr -d ' \t\r\n' < "$REPO_ROOT/VERSION")" -o ../bin/dorkpipe ./cmd/dorkpipe
    fi
    if [[ ! -x "$MCPD_BIN" ]] || paths_newer_than "$MCPD_BIN" "$REPO_ROOT/packages/dorkpipe-mcp" "$REPO_ROOT/VERSION"; then
      try_rebuild_or_keep_existing "$MCPD_BIN" mcpd \
        go_build_in_dir "$REPO_ROOT/packages/dorkpipe-mcp" -trimpath -ldflags "-s -w -X main.Version=$(tr -d ' \t\r\n' < "$REPO_ROOT/VERSION")" -o bin/mcpd ./cmd/mcpd
    fi
    if [[ ! -x "$PIPEON_DESKTOP_BIN" ]] || paths_newer_than "$PIPEON_DESKTOP_BIN" "$REPO_ROOT/src/apps/pipeon-desktop" "$REPO_ROOT/VERSION"; then
      if command -v cargo >/dev/null 2>&1; then
        make build-pipeon-desktop
      fi
    fi
    build_code_server_image_if_needed
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
  exit 1
fi

if [[ ! -f "$PIPEON_DESKTOP_SCRIPT" ]]; then
  echo "pipeon-dev-stack: missing desktop launcher script at $PIPEON_DESKTOP_SCRIPT" >&2
  exit 1
fi

if ! pipeon_stack_compose_running; then
  docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --project-directory "$REPO_ROOT" up -d --remove-orphans
  STACK_STARTED_BY_ME=1
fi

if [[ -f "$PID_FILE" ]]; then
  stale_pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [[ -n "${stale_pid:-}" ]] && ! kill -0 "$stale_pid" 2>/dev/null; then
    rm -f "$PID_FILE"
  fi
fi

if [[ ! -f "$PID_FILE" ]]; then
  MCP_API_KEY="$(cat "$(pipeon_stack_api_key_file)")"
  : > "$LOG_FILE"
  (
    cd "$WORKDIR"
    export DOCKPIPE_BIN
    export DORKPIPE_BIN
    export DOCKPIPE_MCP_TIER="${DOCKPIPE_MCP_TIER:-exec}"
    export DOCKPIPE_MCP_RESTRICT_WORKDIR=1
    export DOCKPIPE_MCP_REQUIRE_ABSOLUTE_BIN=1
    exec "$MCPD_BIN" -http "127.0.0.1:${MCP_PORT}" -api-key "$MCP_API_KEY" -insecure-loopback
  ) >>"$LOG_FILE" 2>&1 &
  MCP_PID=$!
  printf '%s' "$MCP_PID" > "$PID_FILE"
  MCP_STARTED_BY_ME=1
  sleep 1
  if ! kill -0 "$MCP_PID" 2>/dev/null; then
    echo "pipeon-dev-stack: mcpd failed to start; see $LOG_FILE" >&2
    exit 1
  fi
  if ! wait_for_mcp_ready "$MCP_API_KEY" 40; then
    echo "pipeon-dev-stack: mcpd did not become reachable at $MCP_URL; see $LOG_FILE" >&2
    exit 1
  fi
fi

export DATABASE_URL="${DATABASE_URL:-postgresql://dorkpipe:dorkpipe@127.0.0.1:15432/dorkpipe}"
export CODE_SERVER_WAIT="${CODE_SERVER_WAIT:-1}"
export CODE_SERVER_AUTH="${CODE_SERVER_AUTH:-none}"
export CODE_SERVER_CONTAINER_NAME="$CODE_SERVER_CONTAINER_NAME"
export CODE_SERVER_URL="$CODE_SERVER_URL"
export PIPEON_WINDOW_TITLE="${PIPEON_WINDOW_TITLE:-Pipeon}"
export DOCKPIPE_PIPEON="${DOCKPIPE_PIPEON:-1}"
export DOCKPIPE_PIPEON_ALLOW_PRERELEASE="${DOCKPIPE_PIPEON_ALLOW_PRERELEASE:-1}"
export OLLAMA_HOST="${OLLAMA_HOST:-http://172.17.0.1:11434}"
export PIPEON_OLLAMA_MODEL="${PIPEON_OLLAMA_MODEL:-$MODEL_NAME}"
export MCP_HTTP_URL="$MCP_URL"
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
  docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --project-directory "$REPO_ROOT" exec -T ollama ollama pull "$MODEL_NAME" >&2
fi

if [[ "${PIPEON_DEV_STACK_PIPEON_BUNDLE:-1}" == "1" && -x "$PIPEON_BIN" ]]; then
  (
    cd "$WORKDIR"
    export DOCKPIPE_WORKDIR="$WORKDIR"
    "$PIPEON_BIN" bundle
  ) >&2
fi

write_pipeon_stack_runtime_env

cat >&2 <<EOF
[dockpipe-ready] pipeon-dev-stack
[pipeon-dev-stack] ready
  workdir:      $WORKDIR
  ide:          Pipeon
  ui:           $CODE_SERVER_URL
  mcp:          $MCP_URL
  mcp api key:  $(pipeon_stack_api_key_file)
  ollama:       $OLLAMA_HOST
  postgres:     $DATABASE_URL
  state:        $(pipeon_stack_state_dir)
  log:          $LOG_FILE
EOF

bash "$PIPEON_DESKTOP_SCRIPT"
