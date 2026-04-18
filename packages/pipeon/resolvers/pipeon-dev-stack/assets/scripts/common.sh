#!/usr/bin/env bash
set -euo pipefail

pipeon_stack_workdir() {
  local workdir="${DOCKPIPE_WORKDIR:-$PWD}"
  cd "$workdir" && pwd
}

pipeon_stack_repo_root() {
  if [[ -n "${PIPEON_DEV_STACK_PROJECT_DIR:-}" ]]; then
    cd "$PIPEON_DEV_STACK_PROJECT_DIR" && pwd
    return 0
  fi
  pipeon_stack_workdir
}

pipeon_stack_slug() {
  local workdir base
  workdir="$(pipeon_stack_workdir)"
  base="$(basename "$workdir")"
  printf '%s\n' "$base" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9' '-'
}

pipeon_stack_state_dir() {
  if [[ -n "${DOCKPIPE_PACKAGE_STATE_DIR:-}" ]]; then
    printf '%s\n' "$DOCKPIPE_PACKAGE_STATE_DIR"
    return 0
  fi
  local workdir
  workdir="$(pipeon_stack_workdir)"
  printf '%s/bin/.dockpipe/packages/pipeon-dev-stack\n' "$workdir"
}

pipeon_stack_compose_file() {
  local script_dir
  script_dir="$(dockpipe get script_dir)"
  printf '%s/../compose/docker-compose.yml\n' "$script_dir"
}

pipeon_stack_compose_project() {
  printf 'pipeon-dev-%s\n' "$(pipeon_stack_slug)"
}

pipeon_stack_code_server_name() {
  printf 'pipeon-vscode-%s\n' "$(pipeon_stack_slug)"
}

pipeon_stack_code_server_port_file() {
  printf '%s/code-server.port\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_pick_free_port() {
  python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

ensure_pipeon_stack_code_server_port() {
  local port_file port
  port_file="$(pipeon_stack_code_server_port_file)"
  ensure_pipeon_stack_state_dir
  if [[ -s "$port_file" ]]; then
    return 0
  fi
  port="$(pipeon_stack_pick_free_port)"
  printf '%s\n' "$port" > "$port_file"
}

pipeon_stack_code_server_port() {
  ensure_pipeon_stack_code_server_port
  tr -d ' \t\r\n' < "$(pipeon_stack_code_server_port_file)"
}

pipeon_stack_code_server_url() {
  printf 'http://127.0.0.1:%s/\n' "$(pipeon_stack_code_server_port)"
}

pipeon_stack_code_server_home() {
  printf '%s/code-server-home\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_desktop_bin() {
  local workdir repo_root candidate
  workdir="$(pipeon_stack_workdir)"
  repo_root="$(pipeon_stack_repo_root)"

  for candidate in \
    "$workdir/packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop" \
    "$repo_root/packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop"
  do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done

  command -v pipeon-desktop 2>/dev/null || true
}

pipeon_stack_mcp_port() {
  printf '%s\n' "${PIPEON_DEV_STACK_MCP_PORT:-8766}"
}

pipeon_stack_mcp_url() {
  printf 'http://127.0.0.1:%s/mcp\n' "$(pipeon_stack_mcp_port)"
}

pipeon_stack_api_key_file() {
  printf '%s/api-key\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_runtime_env() {
  printf '%s/runtime.env\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_image_stamp_file() {
  printf '%s/code-server-image.stamp\n' "$(pipeon_stack_state_dir)"
}

ensure_pipeon_stack_state_dir() {
  mkdir -p "$(pipeon_stack_state_dir)"
}

ensure_pipeon_stack_api_key() {
  local key_file
  key_file="$(pipeon_stack_api_key_file)"
  ensure_pipeon_stack_state_dir
  if [[ -s "$key_file" ]]; then
    return 0
  fi
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 24 > "$key_file"
  else
    date +%s%N | sha256sum | cut -d' ' -f1 > "$key_file"
  fi
  chmod 600 "$key_file" 2>/dev/null || true
}

write_pipeon_stack_runtime_env() {
  local workdir repo_root api_key_file
  workdir="$(pipeon_stack_workdir)"
  repo_root="$(pipeon_stack_repo_root)"
  api_key_file="$(pipeon_stack_api_key_file)"
  cat > "$(pipeon_stack_runtime_env)" <<EOF
WORKDIR=$workdir
REPO_ROOT=$repo_root
PIPEON_DEV_STACK_WORKDIR=$workdir
PIPEON_DEV_STACK_REPO_ROOT=$repo_root
PIPEON_DEV_STACK_MCP_PORT=$(pipeon_stack_mcp_port)
PIPEON_DEV_STACK_MCP_API_KEY_FILE=$api_key_file
PIPEON_DEV_STACK_DOCKPIPE_BIN=/repo/src/bin/dockpipe
PIPEON_DEV_STACK_DORKPIPE_BIN=/repo/packages/dorkpipe/bin/dorkpipe
PIPEON_DEV_STACK_MCPD_BIN=/repo/packages/dorkpipe/bin/mcpd
PIPEON_DEV_STACK_DORKPIPE_WORKDIR=/work
PIPEON_DEV_STACK_DORKPIPE_DATABASE_URL=postgresql://dorkpipe:dorkpipe@postgres:5432/dorkpipe
PIPEON_DEV_STACK_DORKPIPE_OLLAMA_HOST=http://ollama:11434
DORKPIPE_DEV_STACK_PROJECT=$(pipeon_stack_compose_project)
CODE_SERVER_CONTAINER_NAME=$(pipeon_stack_code_server_name)
CODE_SERVER_URL=$(pipeon_stack_code_server_url)
MCP_HTTP_URL=$(pipeon_stack_mcp_url)
MCP_HTTP_API_KEY_FILE=$api_key_file
EOF
}

pipeon_stack_compose_running() {
  local compose_file project
  compose_file="$(pipeon_stack_compose_file)"
  project="$(pipeon_stack_compose_project)"
  docker compose -p "$project" -f "$compose_file" ps -q 2>/dev/null | grep -q .
}
