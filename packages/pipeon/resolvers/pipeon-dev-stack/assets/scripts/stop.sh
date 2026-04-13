#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/common.sh"

PROJECT_DIR="$(pipeon_stack_repo_root)"
COMPOSE_FILE="$(pipeon_stack_compose_file)"
COMPOSE_PROJECT="$(pipeon_stack_compose_project)"
CODE_SERVER_CONTAINER_NAME="$(pipeon_stack_code_server_name)"

pipeon_stack_stop_mcpd
docker rm -f "$CODE_SERVER_CONTAINER_NAME" >/dev/null 2>&1 || true
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --project-directory "$PROJECT_DIR" down >/dev/null 2>&1 || true

echo "pipeon-dev-stack: removed MCP, code-server, and compose sidecars for $(pipeon_stack_workdir)"
