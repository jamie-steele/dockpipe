#!/usr/bin/env bash
# Host entry: run DorkPipe self-analysis DAG (writes bin/.dockpipe/orchestrator-cursor-prompt.md).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
cd "$ROOT"
export DOCKPIPE_WORKDIR="$ROOT"

resolve_dorkpipe_bin() {
	local configured="${DORKPIPE_BIN:-}"
	local candidate
	if [[ -n "$configured" ]]; then
		printf '%s\n' "$configured"
		return 0
	fi
	for candidate in \
		"$ROOT/packages/dorkpipe/bin/dorkpipe" \
		"$SCRIPT_DIR/../../../../packages/dorkpipe/bin/dorkpipe"
	do
		if [[ -x "$candidate" ]]; then
			printf '%s\n' "$candidate"
			return 0
		fi
	done
	command -v dorkpipe 2>/dev/null || true
}

BIN="$(resolve_dorkpipe_bin)"
SPEC="${DORKPIPE_SELF_ANALYSIS_SPEC:-${SCRIPT_DIR}/../../dorkpipe-self-analysis/spec.yaml}"
if [[ ! -x "$BIN" ]]; then
	echo "dorkpipe-self-analysis: build the orchestrator first: make maintainer-tools (expected $BIN)" >&2
	exit 1
fi
if [[ ! -f "$SPEC" ]]; then
	echo "dorkpipe-self-analysis: missing spec $SPEC" >&2
	exit 1
fi
# spec.combined.yaml needs a local Ollama server (default http://127.0.0.1:11434).
if [[ "$SPEC" == *spec.combined.yaml ]]; then
	OHOST="${OLLAMA_HOST:-http://127.0.0.1:11434}"
	OHOST="${OHOST%/}"
	if command -v curl >/dev/null 2>&1; then
		if ! curl -sf --connect-timeout 2 "${OHOST}/api/tags" >/dev/null; then
			echo "dorkpipe-self-analysis: Ollama not reachable at ${OHOST} (needed for spec.combined.yaml)." >&2
			echo "  Start Ollama (e.g. run the Ollama app or: ollama serve), or set OLLAMA_HOST to your server." >&2
			echo "  Or use the fast path without Ollama: ./packages/dorkpipe/resolvers/dorkpipe/assets/scripts/run-self-analysis.sh" >&2
			exit 1
		fi
	else
		echo "dorkpipe-self-analysis: warning: curl not found; cannot preflight Ollama — combined run may fail" >&2
	fi
fi
"$BIN" run -f "$SPEC" --workdir "$ROOT"
echo ""
echo "dorkpipe-self-analysis: full handoff → ${ROOT}/bin/.dockpipe/orchestrator-cursor-prompt.md"
echo "dorkpipe-self-analysis: raw facts → ${ROOT}/bin/.dockpipe/packages/dorkpipe/self-analysis/"
if [[ -f "${ROOT}/bin/.dockpipe/orchestrator-cursor-prompt.refined.md" ]]; then
	echo "dorkpipe-self-analysis: Ollama refine → ${ROOT}/bin/.dockpipe/orchestrator-cursor-prompt.refined.md"
fi
PASTE="${ROOT}/bin/.dockpipe/paste-this-prompt.txt"
if [[ -f "$PASTE" ]]; then
	echo ""
	echo "========================================================================"
	echo "  COPY-PASTE FOR YOUR AI ASSISTANT (same text in ${PASTE})"
	echo "========================================================================"
	echo ""
	cat "$PASTE"
	echo ""
	echo "========================================================================"
	echo "  (end of paste-this-prompt)"
	echo "========================================================================"
else
	echo "dorkpipe-self-analysis: warning: missing $PASTE" >&2
fi
