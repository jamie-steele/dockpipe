#!/usr/bin/env bash
# After Ollama refine (combined spec), fold refined output into paste-this-prompt.txt.
set -euo pipefail
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
cd "$ROOT"
PASTE="${ROOT}/.dockpipe/paste-this-prompt.txt"
REFINED="${ROOT}/.dockpipe/orchestrator-cursor-prompt.refined.md"
if [[ ! -f "$REFINED" ]]; then
	echo "merge-paste-prompt: no $REFINED; leaving $PASTE unchanged" >&2
	exit 0
fi
if [[ ! -f "$PASTE" ]]; then
	echo "merge-paste-prompt: missing $PASTE" >&2
	exit 1
fi
{
	echo "You are working in the DockPipe repository. Follow AGENTS.md."
	echo "Below: base implementation prompt, then Ollama-refined priorities from this checkout."
	echo ""
	echo "========== BASE PROMPT =========="
	cat "$PASTE"
	echo ""
	echo "========== REFINED PRIORITIES (Ollama) =========="
	cat "$REFINED"
	echo ""
	echo "========== END — paste everything above into your AI assistant =========="
} >"${PASTE}.new"
mv "${PASTE}.new" "$PASTE"
echo "merge-paste-prompt: merged refine into $PASTE"
