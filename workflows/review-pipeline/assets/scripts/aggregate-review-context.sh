#!/usr/bin/env bash
# Build a compact markdown + JSON bundle for the final LLM step from env + collect-go-review-signals outputs.
# Expects merged workflow env (TESTS_PASS, SCAN_PASS, …) and bin/.dockpipe/review-files.txt / review-signals.txt.
set -euo pipefail

ROOT="$(dockpipe get workdir)"
cd "$ROOT"
OUT="${ROOT}/.dockpipe"
mkdir -p "$OUT"

MD="$OUT/review-context.md"
JSON="$OUT/review-context.json"
ENV_SUM="$OUT/review-summary.env"

FILES="$OUT/review-files.txt"
SIGS="$OUT/review-signals.txt"

{
	echo "# Review context (DockPipe prep bundle)"
	echo "Generated for final resolver review — do not re-enumerate the whole repo from scratch."
	echo ""
	echo "## Workflow flags (trust these)"
	echo "- WORKFLOW_NAME=${WORKFLOW_NAME:-}"
	echo "- PREPARE_OK=${PREPARE_OK:-}"
	echo "- TESTS_PASS=${TESTS_PASS:-} TESTS_EXIT=${TESTS_EXIT:-}"
	echo "- SCAN_PASS=${SCAN_PASS:-} VET_EXIT=${VET_EXIT:-}"
	echo "- REVIEW_PREP_OK=${REVIEW_PREP_OK:-}"
	echo "- LOCAL_MODEL_STATUS=${LOCAL_MODEL_STATUS:-}"
	echo ""
	echo "## Go files (first 80 paths; full list in review-files.txt on disk)"
	if [[ -f "$FILES" ]]; then head -n 80 "$FILES"; else echo "(no file list)"; fi
	echo ""
	echo "## Signals (preview; full bounded grep in review-signals.txt on disk)"
	if [[ -f "$SIGS" ]]; then head -n 120 "$SIGS"; else echo "(no signals)"; fi
} >"$MD"

printf '{"workflow":"%s","tests_pass":"%s","scan_pass":"%s","review_prep_ok":"%s"}\n' \
	"${WORKFLOW_NAME:-}" "${TESTS_PASS:-}" "${SCAN_PASS:-}" "${REVIEW_PREP_OK:-}" >"$JSON"

{
	echo "REVIEW_CONTEXT_PATH=bin/.dockpipe/review-context.md"
	echo "REVIEW_JSON_PATH=bin/.dockpipe/review-context.json"
	echo "REVIEW_FILES_LIST=bin/.dockpipe/review-files.txt"
} >"$ENV_SUM"

echo "[review] aggregate-review-context: wrote $MD" >&2
