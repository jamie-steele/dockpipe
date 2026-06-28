#!/usr/bin/env bash
# Local sidecar stack for DorkPipe: control plane + Postgres+pgvector + Ollama (docker compose).
# Usage: scripts/dorkpipe/dev-stack.sh up|down|ps
# Compose file: packages/dorkpipe/resolvers/dorkpipe/assets/compose/docker-compose.yml
# When done: dev-stack.sh down — containers stop (nothing long-lived required for orchestration).
set -euo pipefail
SOURCE_PATH="${0}"
SOURCE_DIR="${SOURCE_PATH%/*}"
[[ "$SOURCE_DIR" == "$SOURCE_PATH" ]] && SOURCE_DIR="."
SCRIPT_DIR="$(cd "$SOURCE_DIR" && pwd)"
INFERRED_REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
PWD_REPO_ROOT="$(pwd)"
GIT_REPO_ROOT="$(git -C "$PWD_REPO_ROOT" rev-parse --show-toplevel 2>/dev/null || true)"
REPO_ROOT=""
for candidate in \
	"${DORKPIPE_DEV_STACK_REPO_ROOT:-}" \
	"${DOCKPIPE_REPO_ROOT:-}" \
	"${DOCKPIPE_WORKDIR:-}" \
	"$GIT_REPO_ROOT" \
	"$PWD_REPO_ROOT" \
	"$INFERRED_REPO_ROOT"
do
	if [[ -n "$candidate" && -f "$candidate/packages/dorkpipe/resolvers/dorkpipe/assets/compose/Dockerfile.dorkpipe-stack" ]]; then
		REPO_ROOT="$(cd "$candidate" && pwd)"
		break
	fi
done
if [[ -z "$REPO_ROOT" ]]; then
	echo "dev-stack: could not find DockPipe repo root for dorkpipe-stack build context" >&2
	exit 1
fi
COMPOSE="$SCRIPT_DIR/../compose/docker-compose.yml"
source "$SCRIPT_DIR/dev-stack-lib.sh"
if [[ ! -f "$COMPOSE" ]]; then
	echo "dev-stack: missing $COMPOSE" >&2
	exit 1
fi
export DORKPIPE_DEV_STACK_REPO_ROOT="${DORKPIPE_DEV_STACK_REPO_ROOT:-$REPO_ROOT}"
export DORKPIPE_DEV_STACK_WORKDIR="${DORKPIPE_DEV_STACK_WORKDIR:-$REPO_ROOT}"
cmd="${1:-}"
PROJECT="${DORKPIPE_DEV_STACK_PROJECT:-dorkpipe-dev}"
GPU_PROMPT_RESULT=""
dorkpipe_stack_bootstrap_sdk >/dev/null 2>&1 || true
mapfile -t COMPOSE_ARGS < <(dorkpipe_stack_compose_args)

dorkpipe_stack_require_local_binaries() {
	local missing=()
	for path in \
		"$REPO_ROOT/src/bin/dockpipe" \
		"$REPO_ROOT/bin/.dockpipe/tooling/bin/dorkpipe" \
		"$REPO_ROOT/bin/.dockpipe/tooling/bin/mcpd"
	do
		if [[ ! -x "$path" ]]; then
			missing+=("$path")
		fi
	done
	if [[ "${#missing[@]}" -eq 0 ]]; then
		return 0
	fi
	echo "dev-stack: dorkpipe-stack image needs local built binaries:" >&2
	printf '  missing: %s\n' "${missing[@]}" >&2
	echo "dev-stack: run: make build && ./src/bin/dockpipe package build source --workdir . --only dorkpipe" >&2
	return 1
}

case "$cmd" in
up)
	dorkpipe_stack_configure_gpu
	case "${DORKPIPE_DEV_STACK_PROMPT_RESULT:-}" in
	gpu-setup)
		echo "dorkpipe-dev-stack: launch paused before starting services so Docker GPU access can be enabled"
		exit 0
		;;
	cancelled)
		echo "dorkpipe-dev-stack: launch cancelled before starting services"
		exit 0
		;;
	esac
	mapfile -t COMPOSE_ARGS < <(dorkpipe_stack_compose_args)
	if [[ -n "${DORKPIPE_DEV_STACK_SERVICES:-}" ]]; then
		# shellcheck disable=SC2206
		SERVICES=(${DORKPIPE_DEV_STACK_SERVICES})
	else
		SERVICES=(postgres ollama dorkpipe-stack dorkpipe-mcp-proxy)
	fi
	if dorkpipe_stack_service_enabled dorkpipe-stack; then
		dorkpipe_stack_require_local_binaries
	fi
	if dorkpipe_stack_service_enabled dorkpipe-stack || dorkpipe_stack_service_enabled dorkpipe-mcp-proxy; then
		dorkpipe_stack_prepare_mcp_material
	fi
	UP_ARGS=(up -d --remove-orphans)
	case "${DORKPIPE_DEV_STACK_RELOAD:-0}" in
	1 | true | yes | on)
		echo "dev-stack: reload requested — rebuilding images and recreating requested services" >&2
		UP_ARGS+=(--build --force-recreate)
		;;
	esac
	docker compose "${COMPOSE_ARGS[@]}" "${UP_ARGS[@]}" "${SERVICES[@]}"
	dorkpipe_stack_ensure_ollama_model
	dorkpipe_stack_wait_for_mcp_ready "${DORKPIPE_DEV_STACK_MCP_READY_ATTEMPTS:-60}"
	if dorkpipe_stack_service_enabled dorkpipe-mcp-proxy; then
		echo "dev-stack: up — MCP $(dorkpipe_stack_mcp_url)  Ollama http://127.0.0.1:11434  Postgres postgresql://dorkpipe:dorkpipe@127.0.0.1:15432/dorkpipe (project $PROJECT; gpu ${DORKPIPE_DEV_STACK_GPU:-auto})"
	else
		echo "dev-stack: up — Ollama http://127.0.0.1:11434  Postgres postgresql://dorkpipe:dorkpipe@127.0.0.1:15432/dorkpipe (project $PROJECT; gpu ${DORKPIPE_DEV_STACK_GPU:-auto})"
	fi
	;;
down)
	docker compose "${COMPOSE_ARGS[@]}" down
	echo "dev-stack: down — sidecar stack stopped"
	;;
ps | status)
	docker compose "${COMPOSE_ARGS[@]}" ps
	;;
*)
	echo "usage: $0 up|down|ps" >&2
	echo "  Brings the DorkPipe MCP control plane, Postgres+pgvector, and Ollama up for local DAG nodes." >&2
	exit 1
	;;
esac
