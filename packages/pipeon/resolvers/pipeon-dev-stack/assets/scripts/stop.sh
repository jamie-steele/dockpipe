#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/common.sh"

PROJECT_DIR="$(pipeon_stack_repo_root)"
COMPOSE_FILE="$(pipeon_stack_compose_file)"
COMPOSE_PROJECT="$(pipeon_stack_compose_project)"
LEGACY_COMPOSE_PROJECT="$(pipeon_stack_legacy_compose_project)"
CODE_SERVER_CONTAINER_NAME="$(pipeon_stack_code_server_name)"
LEGACY_CODE_SERVER_CONTAINER_NAME="$(pipeon_stack_legacy_code_server_name)"
RUNTIME_ENV="$(pipeon_stack_runtime_env)"

compose_cmd() {
  docker compose --env-file "$RUNTIME_ENV" -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --project-directory "$PROJECT_DIR" "$@"
}

docker rm -f "$CODE_SERVER_CONTAINER_NAME" >/dev/null 2>&1 || true
docker rm -f "$LEGACY_CODE_SERVER_CONTAINER_NAME" >/dev/null 2>&1 || true
docker ps -aq --filter "label=com.dockpipe.stack.project=$COMPOSE_PROJECT" | xargs -r docker rm -f >/dev/null 2>&1 || true
docker ps -aq --filter "label=com.dockpipe.stack.project=$LEGACY_COMPOSE_PROJECT" | xargs -r docker rm -f >/dev/null 2>&1 || true
compose_cmd down >/dev/null 2>&1 || true
docker compose --env-file "$RUNTIME_ENV" -p "$LEGACY_COMPOSE_PROJECT" -f "$COMPOSE_FILE" --project-directory "$PROJECT_DIR" down >/dev/null 2>&1 || true

echo "pipeon-dev-stack: removed isolated DorkPipe stack, MCP bridge, and code-server for $(pipeon_stack_workdir)"
