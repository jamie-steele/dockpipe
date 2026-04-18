#!/usr/bin/env bash
# Send one user message to local Ollama with Pipeon system prompt and optional compatibility snapshot.
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
ROOT="$(dockpipe get workdir)"
# shellcheck source=lib/enable.sh
source "$SCRIPT_DIR/lib/enable.sh"

pipeon_check_enabled "$ROOT" || exit $?

OLLAMA_HOST="${OLLAMA_HOST:-http://127.0.0.1:11434}"
OLLAMA_HOST="${OLLAMA_HOST%/}"
MODEL="${PIPEON_OLLAMA_MODEL:-${DOCKPIPE_OLLAMA_MODEL:-llama3.2}}"

QUESTION="${*:-}"
if [[ -z "${QUESTION// }" ]]; then
	if [[ -t 0 ]]; then
		echo "usage: pipeon chat <question>   (or pipe a question on stdin)" >&2
		exit 1
	fi
	QUESTION="$(cat)"
fi

SYS_FILE="$SCRIPT_DIR/prompts/system.md"
if [[ ! -f "$SYS_FILE" ]]; then
	echo "pipeon: missing system prompt $SYS_FILE" >&2
	exit 1
fi

SYS="$(cat "$SYS_FILE")"
# Keep JSON safe: escape backslashes and quotes in user content for jq --arg
COMBINED="$SYS
CTX_FILE="$ROOT/bin/.dockpipe/pipeon-context.md"
if [[ -f "$CTX_FILE" ]]; then
	CTX="$(cat "$CTX_FILE")"
	COMBINED="$COMBINED

---

## Compatibility snapshot (optional)

$CTX"
fi

if ! command -v jq >/dev/null 2>&1; then
	echo "pipeon chat: jq is required" >&2
	exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
	echo "pipeon chat: curl is required" >&2
	exit 1
fi

REQ="$(jq -n \
	--arg model "$MODEL" \
	--arg sys "$COMBINED" \
	--arg user "$QUESTION" \
	'{
		model: $model,
		stream: false,
		messages: [
			{role: "system", content: $sys},
			{role: "user", content: $user}
		]
	}')"

RESP="$(curl -sf "$OLLAMA_HOST/api/chat" -H 'Content-Type: application/json' -d "$REQ")" || {
	echo "pipeon chat: Ollama request failed. Is \`ollama serve\` running at $OLLAMA_HOST ? Model pulled: ollama pull $MODEL" >&2
	exit 1
}
echo "$RESP" | jq -r '.message.content // .response // empty'
echo ""
