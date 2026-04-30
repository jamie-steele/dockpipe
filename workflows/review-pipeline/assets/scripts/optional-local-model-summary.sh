#!/usr/bin/env bash
# Cheap local pass: call Ollama HTTP API (default) or DOCKPIPE_LOCAL_MODEL_CMD override.
# Used after ollama serve (isolate: ollama + run-local-model-with-ollama-daemon.sh) or from a host run:.
# Writes bin/.dockpipe/local-model-notes.txt, local-model.env, and outputs.env for the next step.
set -euo pipefail

ROOT="$(dockpipe get workdir)"
cd "$ROOT"
OUT="${ROOT}/bin/.dockpipe"
mkdir -p "$OUT"
NOTES="$OUT/local-model-notes.txt"
CTX="$OUT/review-context.md"
STATUS="$OUT/local-model.env"
OUT_ENV="$OUT/outputs.env"

OLLAMA_HOST="${OLLAMA_HOST:-http://127.0.0.1:11434}"
OLLAMA_HOST="${OLLAMA_HOST%/}"
MODEL="${DOCKPIPE_OLLAMA_MODEL:-llama3.2}"
DOCKER_NAME="${DOCKPIPE_OLLAMA_CONTAINER_NAME:-dockpipe-ollama-local}"
DOCKER_VOL="${DOCKPIPE_OLLAMA_VOLUME:-dockpipe-ollama}"
DOCKER_IMG="${DOCKPIPE_OLLAMA_IMAGE:-ollama/ollama}"
DOCKER_PORT="${DOCKPIPE_OLLAMA_PORT:-11434}"

write_outputs() {
	local st="$1"
	{
		echo "LOCAL_MODEL_STATUS=$st"
		echo "DEMO_STAGE=local-summary"
	} >"$OUT_ENV"
	echo "LOCAL_MODEL_STATUS=$st" >"$STATUS"
}

if [[ ! -f "$CTX" ]]; then
	echo "Missing review-context.md — run aggregate-review-context.sh first." >"$NOTES"
	write_outputs error
	exit 1
fi

if [[ -n "${DOCKPIPE_LOCAL_MODEL_CMD:-}" ]]; then
	set +e
	# shellcheck disable=SC2086
	eval "$DOCKPIPE_LOCAL_MODEL_CMD" >"$NOTES" 2>&1
	rc=$?
	set -e
	if [[ "$rc" -ne 0 ]]; then
		echo "(exit $rc)" >>"$NOTES"
		write_outputs error
		exit 1
	fi
	write_outputs ran
	exit 0
fi

ollama_ping() {
	curl -sf "$OLLAMA_HOST/api/tags" >/dev/null || curl -sf "$OLLAMA_HOST/api/version" >/dev/null
}

maybe_start_docker_ollama() {
	if [[ "${DOCKPIPE_OLLAMA_DOCKER:-}" != "1" ]]; then
		return 1
	fi
	if ! command -v docker >/dev/null 2>&1; then
		return 1
	fi
	if docker ps --format '{{.Names}}' | grep -qx "$DOCKER_NAME"; then
		return 0
	fi
	docker rm -f "$DOCKER_NAME" 2>/dev/null || true
	# shellcheck disable=SC2086
	docker run -d \
		--name "$DOCKER_NAME" \
		-p "${DOCKER_PORT}:11434" \
		-v "${DOCKER_VOL}:/root/.ollama" \
		"$DOCKER_IMG" >/dev/null
	echo "[dockpipe] local-summary: started Ollama container ${DOCKER_NAME} (DOCKPIPE_OLLAMA_DOCKER=1)" >&2
	return 0
}

wait_for_ollama() {
	local i
	for i in $(seq 1 120); do
		if ollama_ping; then
			return 0
		fi
		sleep 1
	done
	return 1
}

ensure_model() {
	if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$DOCKER_NAME"; then
		echo "[dockpipe] local-summary: ensuring model ${MODEL} in container…" >&2
		docker exec "$DOCKER_NAME" ollama pull "$MODEL" >&2 || true
		return 0
	fi
	if command -v ollama >/dev/null 2>&1; then
		echo "[dockpipe] local-summary: ensuring model ${MODEL}…" >&2
		ollama pull "$MODEL" >&2 || true
	fi
}

json_generate_body() {
	local prompt=$1
	if command -v jq >/dev/null 2>&1; then
		jq -n --arg model "$MODEL" --arg prompt "$prompt" '{model:$model,prompt:$prompt,stream:false}'
		return
	fi
	if command -v python3 >/dev/null 2>&1; then
		export MODEL
		python3 -c 'import json,os,sys; print(json.dumps({"model":os.environ["MODEL"],"prompt":sys.argv[1],"stream":False}))' "$prompt"
		return
	fi
	echo "optional-local-model-summary: need curl + (jq or python3) to call Ollama" >&2
	return 1
}

extract_response() {
	if command -v jq >/dev/null 2>&1; then
		jq -r '.response // empty'
		return
	fi
	python3 -c 'import json,sys; print(json.load(sys.stdin).get("response",""))'
}

run_generate() {
	local prompt chunk body resp
	chunk=$(head -c 12000 "$CTX")
	prompt="Summarize the following review bundle in 5–10 concise bullets for a security/code review prep. Focus on risks, hotspots, and gaps (tests, error handling). Be concrete.

${chunk}"

	body=$(json_generate_body "$prompt") || return 1
	resp=$(curl -sfS "$OLLAMA_HOST/api/generate" -H 'Content-Type: application/json' -d "$body") || return 1
	printf '%s\n' "$resp" | extract_response
}

if ! ollama_ping; then
	if maybe_start_docker_ollama; then
		: # OLLAMA_HOST should match published port (default 11434)
	else
		{
			echo "Ollama not reachable at ${OLLAMA_HOST}"
			echo ""
			echo "Start a daemon, then re-run:"
			echo "  ollama serve"
			echo "Or (Docker):"
			echo "  docker run -d --name ${DOCKER_NAME} -p ${DOCKER_PORT}:11434 -v ${DOCKER_VOL}:/root/.ollama ${DOCKER_IMG}"
			echo "Or set DOCKPIPE_OLLAMA_DOCKER=1 on this step to auto-start that container."
			echo "Override URL: OLLAMA_HOST=http://127.0.0.1:11434"
		} >"$NOTES"
		write_outputs unavailable
		exit 1
	fi
fi

if ! wait_for_ollama; then
	echo "Ollama at ${OLLAMA_HOST} did not become ready in time." >"$NOTES"
	write_outputs error
	exit 1
fi

ensure_model

set +e
text=$(run_generate)
rc=$?
set -e
if [[ "$rc" -ne 0 ]] || [[ -z "${text//[$' \t\n']/}" ]]; then
	ensure_model
	set +e
	text=$(run_generate)
	rc=$?
	set -e
fi

if [[ "$rc" -ne 0 ]] || [[ -z "${text//[$' \t\n']/}" ]]; then
	{
		echo "Ollama generate failed (exit ${rc}). Is model ${MODEL} pulled?"
		echo "Try: ollama pull ${MODEL}"
		echo "Or: docker exec ${DOCKER_NAME} ollama pull ${MODEL}"
	} >"$NOTES"
	write_outputs error
	exit 1
fi

printf '%s\n' "$text" >"$NOTES"
write_outputs ran
exit 0
