#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/common.sh"

REPO_ROOT="$(pipeon_stack_repo_root)"
STATE_DIR="$(pipeon_stack_state_dir)"
COMPOSE_FILE="$(pipeon_stack_compose_file)"
COMPOSE_PROJECT="$(pipeon_stack_compose_project)"
PID_FILE="$(pipeon_stack_pid_file)"
RUNTIME_ENV="$(pipeon_stack_runtime_env)"
CODE_SERVER_CONTAINER_NAME="$(pipeon_stack_code_server_name)"

echo "pipeon-dev-stack status"
echo "  workdir: $(pipeon_stack_workdir)"
echo "  repo:    $REPO_ROOT"
echo "  state:   $STATE_DIR"
echo "  mcp url: $(pipeon_stack_mcp_url)"
echo "  mcp pid: $(if [[ -f "$PID_FILE" ]]; then cat "$PID_FILE"; else echo not-running; fi)"
echo "  code-server container: $CODE_SERVER_CONTAINER_NAME"

if [[ -f "$RUNTIME_ENV" ]]; then
  echo
  echo "runtime env"
  cat "$RUNTIME_ENV"
fi

echo
echo "compose"
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --project-directory "$REPO_ROOT" ps || true

echo
echo "docker"
docker ps --filter "name=$CODE_SERVER_CONTAINER_NAME" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' || true
