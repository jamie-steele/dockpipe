#!/usr/bin/env bash
# Local sidecar stack for DorkPipe: Postgres+pgvector + Ollama (docker compose).
# Usage: scripts/dorkpipe/dev-stack.sh up|down|ps
# Compose file: packages/dorkpipe/resolvers/dorkpipe/assets/compose/docker-compose.yml
# When done: dev-stack.sh down — containers stop (nothing long-lived required for orchestration).
set -euo pipefail
SOURCE_PATH="${0}"
SOURCE_DIR="${SOURCE_PATH%/*}"
[[ "$SOURCE_DIR" == "$SOURCE_PATH" ]] && SOURCE_DIR="."
SCRIPT_DIR="$(cd "$SOURCE_DIR" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
COMPOSE="$SCRIPT_DIR/../compose/docker-compose.yml"
source "$SCRIPT_DIR/dev-stack-lib.sh"
if [[ ! -f "$COMPOSE" ]]; then
	echo "dev-stack: missing $COMPOSE" >&2
	exit 1
fi
cmd="${1:-}"
PROJECT="${DORKPIPE_DEV_STACK_PROJECT:-dorkpipe-dev}"
GPU_PROMPT_RESULT=""
dorkpipe_stack_bootstrap_sdk >/dev/null 2>&1 || true
mapfile -t COMPOSE_ARGS < <(dorkpipe_stack_compose_args)
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
		SERVICES=(postgres ollama)
	fi
	docker compose "${COMPOSE_ARGS[@]}" up -d --remove-orphans "${SERVICES[@]}"
	echo "dev-stack: up — Ollama http://127.0.0.1:11434  Postgres postgresql://dorkpipe:dorkpipe@127.0.0.1:15432/dorkpipe (project $PROJECT; gpu ${DORKPIPE_DEV_STACK_GPU:-auto})"
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
	echo "  Brings Postgres+pgvector and Ollama up for local DAG nodes (DATABASE_URL, OLLAMA_HOST)." >&2
	exit 1
	;;
esac
