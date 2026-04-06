#!/usr/bin/env bash
set -euo pipefail

pipeon_stack_workdir() {
  local workdir="${DOCKPIPE_WORKDIR:-$PWD}"
  cd "$workdir" && pwd
}

pipeon_stack_repo_root() {
  local workdir
  workdir="$(pipeon_stack_workdir)"
  if git -C "$workdir" rev-parse --show-toplevel >/dev/null 2>&1; then
    git -C "$workdir" rev-parse --show-toplevel
    return 0
  fi
  if [[ -f "$workdir/VERSION" && -d "$workdir/packages" && -d "$workdir/src" ]]; then
    printf '%s\n' "$workdir"
    return 0
  fi
  printf 'pipeon-dev-stack: could not determine repo root from %s\n' "$workdir" >&2
  exit 1
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
  local repo_root
  repo_root="$(pipeon_stack_repo_root)"
  printf '%s/packages/dorkpipe/resolvers/dorkpipe/assets/compose/docker-compose.yml\n' "$repo_root"
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
  local repo_root
  repo_root="$(pipeon_stack_repo_root)"
  printf '%s/src/apps/pipeon-desktop/bin/pipeon-desktop\n' "$repo_root"
}

pipeon_stack_mcp_port() {
  printf '%s\n' "${PIPEON_DEV_STACK_MCP_PORT:-8765}"
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

pipeon_stack_pid_file() {
  printf '%s/mcpd.pid\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_log_file() {
  printf '%s/mcpd.log\n' "$(pipeon_stack_state_dir)"
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
  local workdir repo_root api_key_file api_key
  workdir="$(pipeon_stack_workdir)"
  repo_root="$(pipeon_stack_repo_root)"
  api_key_file="$(pipeon_stack_api_key_file)"
  api_key="$(cat "$api_key_file")"
  cat > "$(pipeon_stack_runtime_env)" <<EOF
WORKDIR=$workdir
REPO_ROOT=$repo_root
DORKPIPE_DEV_STACK_PROJECT=$(pipeon_stack_compose_project)
CODE_SERVER_CONTAINER_NAME=$(pipeon_stack_code_server_name)
CODE_SERVER_URL=$(pipeon_stack_code_server_url)
MCP_HTTP_URL=$(pipeon_stack_mcp_url)
MCP_HTTP_API_KEY=$api_key
MCP_HTTP_API_KEY_FILE=$api_key_file
OLLAMA_HOST=${OLLAMA_HOST:-http://172.17.0.1:11434}
DATABASE_URL=${DATABASE_URL:-postgresql://dorkpipe:dorkpipe@127.0.0.1:15432/dorkpipe}
EOF
}

pipeon_stack_compose_running() {
  local compose_file project
  compose_file="$(pipeon_stack_compose_file)"
  project="$(pipeon_stack_compose_project)"
  docker compose -p "$project" -f "$compose_file" ps -q 2>/dev/null | grep -q .
}

pipeon_stack_stop_mcpd() {
  local pid_file pid
  pid_file="$(pipeon_stack_pid_file)"
  if [[ ! -f "$pid_file" ]]; then
    return 0
  fi
  pid="$(cat "$pid_file" 2>/dev/null || true)"
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  rm -f "$pid_file"
}
