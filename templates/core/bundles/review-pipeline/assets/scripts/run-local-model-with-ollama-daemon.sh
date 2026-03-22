#!/usr/bin/env bash
# Run inside dockpipe isolate: ollama — starts `ollama serve`, waits for HTTP, then local summary.
set -euo pipefail
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
cd "$ROOT"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export OLLAMA_HOST="${OLLAMA_HOST:-http://127.0.0.1:11434}"
OLLAMA_HOST="${OLLAMA_HOST%/}"

if [[ "${DOCKPIPE_OLLAMA_SKIP_SERVE:-}" == "1" ]]; then
	bash "$HERE/optional-local-model-summary.sh"
	exit $?
fi

if ! command -v ollama >/dev/null 2>&1; then
	echo "run-local-model-with-ollama-daemon: ollama binary not found in PATH" >&2
	exit 1
fi

ollama serve &
OLLAMA_PID=$!
cleanup() { kill "${OLLAMA_PID}" 2>/dev/null || true; }
trap cleanup EXIT

for _ in $(seq 1 120); do
	if curl -sf "${OLLAMA_HOST}/api/tags" >/dev/null || curl -sf "${OLLAMA_HOST}/api/version" >/dev/null; then
		break
	fi
	sleep 1
done

if ! curl -sf "${OLLAMA_HOST}/api/tags" >/dev/null && ! curl -sf "${OLLAMA_HOST}/api/version" >/dev/null; then
	echo "run-local-model-with-ollama-daemon: Ollama did not become ready at ${OLLAMA_HOST}" >&2
	exit 1
fi

bash "$HERE/optional-local-model-summary.sh"
exit $?
