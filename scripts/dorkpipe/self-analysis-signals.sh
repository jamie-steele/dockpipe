#!/usr/bin/env bash
# Search-based signals (grounded). Writes .dorkpipe/self-analysis/signals_*.txt
set -euo pipefail
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
OUT="$ROOT/.dorkpipe/self-analysis"
mkdir -p "$OUT"

todo_hits() {
	if command -v rg >/dev/null 2>&1; then
		rg -n 'TODO|FIXME|XXX' "$ROOT/lib/dorkpipe" "$ROOT/scripts/dorkpipe" 2>/dev/null | head -50 || true
	else
		grep -R -n -E 'TODO|FIXME|XXX' "$ROOT/lib/dorkpipe" "$ROOT/scripts/dorkpipe" 2>/dev/null | head -50 || true
	fi
}

engine_files() {
	if command -v rg >/dev/null 2>&1; then
		rg -l 'branch_judge|retrieve_if|EarlyStop|verifier|ShouldEscalate|mergeVectors|dorkpipe\.metrics' \
			"$ROOT/lib/dorkpipe" "$ROOT/dockpipe-experimental/workflows" "$ROOT/scripts/dorkpipe" 2>/dev/null | sort -u | head -80 || true
	else
		grep -R -l -E 'branch_judge|retrieve_if|EarlyStop|verifier' "$ROOT/lib/dorkpipe" 2>/dev/null | head -80 || true
	fi
}

{
	echo "### TODO/FIXME/XXX in lib/dorkpipe and scripts/dorkpipe (first 50)"
	todo_hits
} >"$OUT/signals_todo.txt"

{
	echo "### Orchestration keyword files"
	engine_files
} >"$OUT/signals_engine_files.txt"

{
	echo "### spec.example.yaml (orchestrator) excerpt"
	f="$ROOT/dockpipe-experimental/workflows/dorkpipe-orchestrator/spec.example.yaml"
	if [[ -f "$f" ]]; then
		sed -n '1,80p' "$f"
	fi
} >"$OUT/signals_spec_example_excerpt.txt"

{
	echo "### Recent git log"
	git -C "$ROOT" log -8 --oneline 2>/dev/null || true
} >"$OUT/signals_git_log.txt"

{
	echo "### go list (lib/dorkpipe/...)"
	if command -v go >/dev/null 2>&1; then
		(cd "$ROOT" && go list ./lib/dorkpipe/... 2>/dev/null) || true
	fi
} >"$OUT/signals_go_list.txt"

if [[ -f "$ROOT/.dorkpipe/metrics.jsonl" ]]; then
	tail -5 "$ROOT/.dorkpipe/metrics.jsonl" >"$OUT/signals_metrics_tail.txt" || true
else
	echo "(no .dorkpipe/metrics.jsonl yet — run dorkpipe eval after orchestrator runs)" >"$OUT/signals_metrics_tail.txt"
fi

echo "self-analysis-signals: wrote $OUT/signals_*.txt"
