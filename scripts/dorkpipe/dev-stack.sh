#!/usr/bin/env bash
# Local sidecar stack for DorkPipe: Postgres+pgvector + Ollama (docker compose).
# Usage: scripts/dorkpipe/dev-stack.sh up|down|ps
# Compose file: templates/core/assets/compose/dorkpipe/docker-compose.yml
# When done: dev-stack.sh down — containers stop (nothing long-lived required for orchestration).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
COMPOSE="$REPO_ROOT/templates/core/assets/compose/dorkpipe/docker-compose.yml"
if [[ ! -f "$COMPOSE" ]]; then
	echo "dev-stack: missing $COMPOSE" >&2
	exit 1
fi
cmd="${1:-}"
PROJECT="${DORKPIPE_DEV_STACK_PROJECT:-dorkpipe-dev}"
case "$cmd" in
up)
	docker compose -p "$PROJECT" -f "$COMPOSE" --project-directory "$REPO_ROOT" up -d --remove-orphans
	echo "dev-stack: up — Ollama http://127.0.0.1:11434  Postgres postgresql://dorkpipe:dorkpipe@127.0.0.1:15432/dorkpipe (project $PROJECT)"
	;;
down)
	docker compose -p "$PROJECT" -f "$COMPOSE" --project-directory "$REPO_ROOT" down
	echo "dev-stack: down — sidecar stack stopped"
	;;
ps | status)
	docker compose -p "$PROJECT" -f "$COMPOSE" --project-directory "$REPO_ROOT" ps
	;;
*)
	echo "usage: $0 up|down|ps" >&2
	echo "  Brings Postgres+pgvector and Ollama up for local DAG nodes (DATABASE_URL, OLLAMA_HOST)." >&2
	exit 1
	;;
esac
