#!/usr/bin/env bash
# Local sidecar stack for DorkPipe: control plane + Postgres+pgvector + Ollama (docker compose).
# Usage: scripts/dorkpipe/dev-stack.sh up|down|ps
# Compose file: packages/dorkpipe/resolvers/dorkpipe/assets/compose/docker-compose.yml
# When done: dev-stack.sh down — containers stop (nothing long-lived required for orchestration).
set -euo pipefail
eval "$(dockpipe sdk)"
dockpipe_sdk init-script
SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
ASSETS_DIR="${DOCKPIPE_ASSETS_DIR:-$(cd "${SCRIPT_DIR}/.." && pwd)}"
ROOT="${ROOT:-$(dockpipe_sdk get workdir)}"
COMPOSE="$SCRIPT_DIR/../compose/docker-compose.yml"
source "$SCRIPT_DIR/dev-stack-lib.sh"
if [[ ! -f "$COMPOSE" ]]; then
	echo "dev-stack: missing $COMPOSE" >&2
	exit 1
fi
export DORKPIPE_DEV_STACK_ASSETS_DIR="${DORKPIPE_DEV_STACK_ASSETS_DIR:-$ASSETS_DIR}"
export DORKPIPE_DEV_STACK_CONTEXT_DIR="${DORKPIPE_DEV_STACK_CONTEXT_DIR:-$ASSETS_DIR}"
export DORKPIPE_DEV_STACK_POLICY_PROXY_JS="${DORKPIPE_DEV_STACK_POLICY_PROXY_JS:-$ASSETS_DIR/compose/policy-proxy.js}"
export DORKPIPE_DEV_STACK_WORKDIR="${DORKPIPE_DEV_STACK_WORKDIR:-$ROOT}"
cmd="${1:-}"
PROJECT="${DORKPIPE_DEV_STACK_PROJECT:-dorkpipe-dev}"
mapfile -t COMPOSE_ARGS < <(dorkpipe_stack_compose_args)

ensure_executable_binary() {
	local path="$1"
	if [[ ! -e "$path" ]]; then
		return 1
	fi
	if [[ -x "$path" ]]; then
		return 0
	fi
	# Tarball extraction on Windows/MSYS can preserve file contents but lose the execute bit.
	chmod +x "$path" 2>/dev/null || true
	[[ -x "$path" ]]
}

dorkpipe_stack_require_present_file() {
	local path="$1"
	[[ -f "$path" ]]
}

dorkpipe_stack_require_consumer_binaries() {
	local bundle_mode repo_root context_root missing=()
	bundle_mode="${DORKPIPE_DEV_STACK_BUNDLE_MODE:-package}"
	if [[ "$bundle_mode" == "checkout" ]]; then
		repo_root="$(git -C "$ROOT" rev-parse --show-toplevel 2>/dev/null || true)"
		if [[ -z "$repo_root" || ! -f "$repo_root/packages/dorkpipe/resolvers/dorkpipe/assets/compose/Dockerfile.dorkpipe-stack" ]]; then
			echo "dev-stack: checkout bundle mode requires a DockPipe source checkout" >&2
			return 1
		fi
		for path in \
			"$repo_root/src/bin/dockpipe" \
			"$repo_root/bin/.dockpipe/tooling/bin/dorkpipe" \
			"$repo_root/bin/.dockpipe/tooling/bin/mcpd"
		do
			if ! dorkpipe_stack_require_present_file "$path"; then
				missing+=("$path")
			fi
		done
		if [[ "${#missing[@]}" -gt 0 ]]; then
			echo "dev-stack: checkout bundle mode needs local built binaries:" >&2
			printf '  missing: %s\n' "${missing[@]}" >&2
			echo "dev-stack: run: make build && ./src/bin/dockpipe package build source --workdir . --only dorkpipe" >&2
			return 1
		fi
		context_root="$(dorkpipe_stack_state_dir)/checkout-context"
		rm -rf "$context_root"
		mkdir -p "$context_root/compose" "$context_root/tooling/bin/linux"
		cp "$ASSETS_DIR/compose/Dockerfile.dorkpipe-stack" "$context_root/compose/Dockerfile.dorkpipe-stack"
		if [[ -f "$ASSETS_DIR/compose/Dockerfile.dorkpipe-stack.dockerignore" ]]; then
			cp "$ASSETS_DIR/compose/Dockerfile.dorkpipe-stack.dockerignore" "$context_root/compose/Dockerfile.dorkpipe-stack.dockerignore"
		fi
		cp "$repo_root/src/bin/dockpipe" "$context_root/tooling/bin/linux/dockpipe"
		cp "$repo_root/bin/.dockpipe/tooling/bin/dorkpipe" "$context_root/tooling/bin/linux/dorkpipe"
		cp "$repo_root/bin/.dockpipe/tooling/bin/mcpd" "$context_root/tooling/bin/linux/mcpd"
		chmod +x "$context_root/tooling/bin/linux/dockpipe" "$context_root/tooling/bin/linux/dorkpipe" "$context_root/tooling/bin/linux/mcpd"
		export DORKPIPE_DEV_STACK_CONTEXT_DIR="$context_root"
		return 0
	fi

	for path in \
		"$DORKPIPE_DEV_STACK_CONTEXT_DIR/tooling/bin/linux/dockpipe" \
		"$DORKPIPE_DEV_STACK_CONTEXT_DIR/tooling/bin/linux/dorkpipe" \
		"$DORKPIPE_DEV_STACK_CONTEXT_DIR/tooling/bin/linux/mcpd"
	do
		if ! dorkpipe_stack_require_present_file "$path"; then
			missing+=("$path")
		fi
	done
	if [[ "${#missing[@]}" -eq 0 ]]; then
		return 0
	fi
	echo "dev-stack: packaged consumer binaries are missing from the dorkpipe workflow assets:" >&2
	printf '  missing: %s\n' "${missing[@]}" >&2
	echo "dev-stack: consumer path expects compiled package assets; maintainer overrides:" >&2
	echo "  compile package assets: dockpipe package compile resolvers --workdir \"$ROOT\" --from packages/dorkpipe --force" >&2
	echo "  or use checkout binaries explicitly: DORKPIPE_DEV_STACK_BUNDLE_MODE=checkout scripts/dorkpipe/dev-stack.sh up" >&2
	return 1
}

case "$cmd" in
up)
	if dorkpipe_stack_configure_gpu; then
		:
	else
		gpu_status=$?
		case "$gpu_status" in
		20)
			echo "dorkpipe-dev-stack: launch stopped before starting services because Docker GPU access still requires remediation" >&2
			;;
		21)
			echo "dorkpipe-dev-stack: launch cancelled before starting services" >&2
			;;
		esac
		exit "$gpu_status"
	fi
	mapfile -t COMPOSE_ARGS < <(dorkpipe_stack_compose_args)
	if [[ -n "${DORKPIPE_DEV_STACK_SERVICES:-}" ]]; then
		# shellcheck disable=SC2206
		SERVICES=(${DORKPIPE_DEV_STACK_SERVICES})
	else
		SERVICES=(postgres ollama dorkpipe-stack dorkpipe-mcp-proxy)
	fi
	if dorkpipe_stack_service_enabled dorkpipe-stack || dorkpipe_stack_service_enabled dorkpipe-mcp-proxy; then
		dorkpipe_stack_require_consumer_binaries
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
	dorkpipe_stack_wait_for_services_ready "${DORKPIPE_DEV_STACK_READY_ATTEMPTS:-60}"
	dorkpipe_stack_ensure_ollama_model
	dorkpipe_stack_wait_for_mcp_ready "${DORKPIPE_DEV_STACK_MCP_READY_ATTEMPTS:-60}"
	if dorkpipe_stack_service_enabled dorkpipe-mcp-proxy; then
		echo "dev-stack: up — MCP $(dorkpipe_stack_mcp_url)  Ollama http://127.0.0.1:11434  Postgres postgresql://dorkpipe:dorkpipe@127.0.0.1:15432/dorkpipe (project $PROJECT; gpu ${DORKPIPE_DEV_STACK_GPU:-auto})"
	else
		echo "dev-stack: up — Ollama http://127.0.0.1:11434  Postgres postgresql://dorkpipe:dorkpipe@127.0.0.1:15432/dorkpipe (project $PROJECT; gpu ${DORKPIPE_DEV_STACK_GPU:-auto})"
	fi
	;;
down)
	if dorkpipe_stack_service_enabled dorkpipe-stack || dorkpipe_stack_service_enabled dorkpipe-mcp-proxy; then
		dorkpipe_stack_prepare_mcp_material
	fi
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
