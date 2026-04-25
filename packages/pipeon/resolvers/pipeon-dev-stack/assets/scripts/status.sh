#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/common.sh"

PROJECT_DIR="$(pipeon_stack_repo_root)"
STATE_DIR="$(pipeon_stack_state_dir)"
COMPOSE_FILE="$(pipeon_stack_compose_file)"
COMPOSE_PROJECT="$(pipeon_stack_compose_project)"
RUNTIME_ENV="$(pipeon_stack_runtime_env)"
CODE_SERVER_CONTAINER_NAME="$(pipeon_stack_code_server_name)"

compose_cmd() {
  docker compose --env-file "$RUNTIME_ENV" -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --project-directory "$PROJECT_DIR" "$@"
}

echo "pipeon-dev-stack status"
echo "  workdir: $(pipeon_stack_workdir)"
echo "  project: $PROJECT_DIR"
echo "  state:   $STATE_DIR"
echo "  mcp url: $(pipeon_stack_mcp_url)"
echo "  mcp service: dorkpipe-stack"
echo "  code-server container: $CODE_SERVER_CONTAINER_NAME"

if [[ -f "$RUNTIME_ENV" ]]; then
  echo
  echo "runtime env"
  sed -E 's/^(PIPEON_DEV_STACK_MCP_API_KEY|MCP_HTTP_API_KEY)=.*/\1=<redacted>/' "$RUNTIME_ENV"
fi

echo
echo "compose"
compose_cmd ps || true

echo
echo "docker"
docker ps --filter "name=$CODE_SERVER_CONTAINER_NAME" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' || true
