#!/usr/bin/env bash
# Host entry: run DorkPipe self-analysis DAG (writes package-scoped handoff artifacts).
set -euo pipefail
SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/lib/dorkpipe-cli.sh"
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
cd "$ROOT"
BIN="$(dorkpipe_script_resolve_bin "$(dorkpipe_script_repo_root "$SCRIPT_DIR")")"
SPEC="${DORKPIPE_SELF_ANALYSIS_SPEC:-${SCRIPT_DIR}/../../dorkpipe-self-analysis/spec.yaml}"
if [[ ! -x "$BIN" ]]; then
	echo "dorkpipe-self-analysis: dorkpipe CLI not available from packaged assets, repo-local builds, or PATH (expected $BIN)" >&2
	echo "dorkpipe-self-analysis: consumer path expects compiled package assets or an installed dorkpipe binary; maintainer fallback: dockpipe package build source --workdir . --only dorkpipe" >&2
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
			echo "  Or use the host workflow path: DORKPIPE_SELF_ANALYSIS_SPEC=packages/dorkpipe/resolvers/dorkpipe-self-analysis/spec.combined.yaml dockpipe --workflow dorkpipe-self-analysis-host --workdir . --" >&2
			exit 1
		fi
	else
		echo "dorkpipe-self-analysis: warning: curl not found; cannot preflight Ollama — combined run may fail" >&2
	fi
fi
"$BIN" run -f "$SPEC" --workdir "$ROOT"
echo ""
DOCKPIPE_CLI="${DOCKPIPE_BIN:-${ROOT}/src/bin/dockpipe}"
if [[ -x "$DOCKPIPE_CLI" ]]; then
	HANDOFF_PATH="$("$DOCKPIPE_CLI" scope --package dorkpipe handoff/orchestrator-cursor-prompt.md --workdir "$ROOT")"
	REFINED_PATH="$("$DOCKPIPE_CLI" scope --package dorkpipe handoff/orchestrator-cursor-prompt.refined.md --workdir "$ROOT")"
	PASTE="$("$DOCKPIPE_CLI" scope --package dorkpipe handoff/paste-this-prompt.txt --workdir "$ROOT")"
	echo "dorkpipe-self-analysis: full handoff → ${HANDOFF_PATH}"
	echo "dorkpipe-self-analysis: raw facts → $("$DOCKPIPE_CLI" scope --package dorkpipe self-analysis --workdir "$ROOT")"
else
	HANDOFF_PATH=""
	REFINED_PATH=""
	PASTE=""
	echo "dorkpipe-self-analysis: full handoff → run 'dockpipe scope --package dorkpipe handoff/orchestrator-cursor-prompt.md --workdir \"$ROOT\"'"
	echo "dorkpipe-self-analysis: raw facts → run 'dockpipe scope --package dorkpipe self-analysis --workdir \"$ROOT\"'"
fi
if [[ -n "$REFINED_PATH" && -f "$REFINED_PATH" ]]; then
	echo "dorkpipe-self-analysis: Ollama refine → ${REFINED_PATH}"
fi
if [[ -n "$PASTE" && -f "$PASTE" ]]; then
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
elif [[ -n "$PASTE" ]]; then
	echo "dorkpipe-self-analysis: warning: missing $PASTE" >&2
fi
