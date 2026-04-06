#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/common.sh"

WORKDIR="$(pipeon_stack_workdir)"
REPO_ROOT="$(pipeon_stack_repo_root)"
CODE_SERVER_CONTAINER_NAME="${CODE_SERVER_CONTAINER_NAME:-$(pipeon_stack_code_server_name)}"
CODE_SERVER_PORT="$(pipeon_stack_code_server_port)"
CODE_SERVER_URL="${CODE_SERVER_URL:-$(pipeon_stack_code_server_url)}"
CODE_SERVER_HOME="$(pipeon_stack_code_server_home)"
CODE_SERVER_IMAGE="${CODE_SERVER_IMAGE:-dockpipe-code-server:latest}"
CODE_SERVER_AUTH="${CODE_SERVER_AUTH:-none}"
PIPEON_DESKTOP_BIN="${PIPEON_DESKTOP_BIN:-$(pipeon_stack_desktop_bin)}"
PIPEON_WINDOW_TITLE="${PIPEON_WINDOW_TITLE:-Pipeon}"
WAIT_FOR_UI="${CODE_SERVER_WAIT:-1}"
MCP_HTTP_API_KEY="$(cat "$(pipeon_stack_api_key_file)")"

pipeon_wait_for_http() {
  local url="$1"
  local attempts="${2:-120}"
  local i
  for ((i = 0; i < attempts; i++)); do
    if curl -fsS -I "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}

pipeon_start_code_server() {
  local cid
  mkdir -p "$CODE_SERVER_HOME"
  if docker ps --format '{{.Names}}' | grep -qx "$CODE_SERVER_CONTAINER_NAME"; then
    return 0
  fi
  if docker ps -a --format '{{.Names}}' | grep -qx "$CODE_SERVER_CONTAINER_NAME"; then
    docker rm -f "$CODE_SERVER_CONTAINER_NAME" >/dev/null 2>&1 || true
  fi

  cid="$(
    docker run -d \
    --name "$CODE_SERVER_CONTAINER_NAME" \
    --entrypoint /bin/bash \
    --add-host=host.docker.internal:host-gateway \
    -p "127.0.0.1:${CODE_SERVER_PORT}:8080" \
    -e HOME=/home/coder \
    -e XDG_CACHE_HOME=/home/coder/.cache \
    -e XDG_CONFIG_HOME=/home/coder/.config \
    -e XDG_DATA_HOME=/home/coder/.local/share \
    -e DOTNET_CLI_HOME=/home/coder/.dotnet \
    -e GOCACHE=/home/coder/.cache/go-build \
    -e GIT_CONFIG_GLOBAL=/home/coder/.gitconfig \
    -e DOCKPIPE_PIPEON="${DOCKPIPE_PIPEON:-1}" \
    -e DOCKPIPE_PIPEON_ALLOW_PRERELEASE="${DOCKPIPE_PIPEON_ALLOW_PRERELEASE:-1}" \
    -e DOCKPIPE_WORKDIR=/work \
    -e OLLAMA_HOST="${OLLAMA_HOST:-http://172.17.0.1:11434}" \
    -e PIPEON_OLLAMA_MODEL="${PIPEON_OLLAMA_MODEL:-llama3.2}" \
    -e MCP_HTTP_URL="${MCP_HTTP_URL:-}" \
    -e MCP_HTTP_API_KEY="$MCP_HTTP_API_KEY" \
    -v "$WORKDIR:/work" \
    -v "$CODE_SERVER_HOME:/home/coder" \
    "$CODE_SERVER_IMAGE" \
    -lc '
      set -e
      mkdir -p /home/coder/.local/share/code-server/User
      if [[ ! -f /home/coder/.local/share/code-server/User/settings.json ]] && [[ -f /opt/pipeon/default-user-data/User/settings.json ]]; then
        cp /opt/pipeon/default-user-data/User/settings.json /home/coder/.local/share/code-server/User/settings.json
      fi
      exec code-server \
        --bind-addr 0.0.0.0:8080 \
        --auth "'"$CODE_SERVER_AUTH"'" \
        --user-data-dir /home/coder/.local/share/code-server \
        --extensions-dir /opt/pipeon/extensions \
        /work
    '
  )"

  sleep 1
  if ! docker ps --format '{{.Names}}' | grep -qx "$CODE_SERVER_CONTAINER_NAME"; then
    echo "pipeon-dev-stack: code-server container exited during startup" >&2
    docker logs "$CODE_SERVER_CONTAINER_NAME" >&2 || true
    return 1
  fi
}

if [[ ! -x "$PIPEON_DESKTOP_BIN" ]]; then
  echo "pipeon-dev-stack: Pipeon desktop binary not found at $PIPEON_DESKTOP_BIN" >&2
  echo "Build it with: make build-pipeon-desktop" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "pipeon-dev-stack: curl is required to wait for the Pipeon UI" >&2
  exit 1
fi

pipeon_start_code_server

if [[ "$WAIT_FOR_UI" == "1" ]]; then
  if ! pipeon_wait_for_http "$CODE_SERVER_URL" 120; then
    echo "pipeon-dev-stack: Pipeon UI did not become reachable at $CODE_SERVER_URL" >&2
    docker logs "$CODE_SERVER_CONTAINER_NAME" >&2 || true
    exit 1
  fi
fi

printf '[pipeon-dev-stack] opening Pipeon desktop shell at %s\n' "$CODE_SERVER_URL" >&2
PIPEON_URL="$CODE_SERVER_URL" PIPEON_WINDOW_TITLE="$PIPEON_WINDOW_TITLE" exec "$PIPEON_DESKTOP_BIN"
