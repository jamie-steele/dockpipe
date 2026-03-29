#!/usr/bin/env bash
# Deterministic repo facts for DorkPipe self-analysis (no LLM). Writes .dorkpipe/self-analysis/
set -euo pipefail
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
OUT="$ROOT/.dorkpipe/self-analysis"
mkdir -p "$OUT"

{
	echo "## git"
	git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo "(not a git repo)"
	git -C "$ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || true
	git -C "$ROOT" status -sb 2>/dev/null | head -20 || true
} >"$OUT/git.txt"

if [[ -d "$ROOT/src/lib/dorkpipe" ]]; then
	find "$ROOT/src/lib/dorkpipe" -mindepth 1 -maxdepth 1 -type d | sort | while read -r d; do
		n=$(find "$d" -name '*.go' 2>/dev/null | wc -l | tr -d ' ')
		printf '%s\t%s\n' "$(basename "$d")" "$n"
	done >"$OUT/dorkpipe_packages.tsv"
else
	: >"$OUT/dorkpipe_packages.tsv"
fi

find "$ROOT/src/lib/dorkpipe" -name '*.go' 2>/dev/null | wc -l | tr -d ' ' >"$OUT/dorkpipe_go_files.count" || echo 0 >"$OUT/dorkpipe_go_files.count"

{
	echo "## cmd"
	find "$ROOT/src/cmd" -name '*.go' 2>/dev/null | head -40
} >"$OUT/cmd_go_files.txt" || true

{
	echo "## scripts/dorkpipe"
	ls -la "$ROOT/src/scripts/dorkpipe" 2>/dev/null || true
} >"$OUT/scripts_dorkpipe_ls.txt"

{
	echo "## .staging/packages/dockpipe/agent/dorkpipe (sample)"
	find "$ROOT/.staging/packages/dockpipe/agent/dorkpipe" -type f 2>/dev/null | sort | head -60
} >"$OUT/assets_dorkpipe_files.txt" || true

{
	echo "## key file line counts"
	for f in \
		src/lib/dorkpipe/engine/run.go \
		src/lib/dorkpipe/spec/spec.go \
		src/lib/dorkpipe/aggregator/merge.go \
		src/lib/dorkpipe/workers/workers.go \
		src/cmd/dorkpipe/main.go; do
		if [[ -f "$ROOT/$f" ]]; then
			wc -l "$ROOT/$f"
		fi
	done
} >"$OUT/key_file_wc.txt" || true

{
	echo "## workflows/"
	find "$ROOT/workflows" -name 'config.yml' 2>/dev/null | sort
} >"$OUT/workflow_configs.txt" || true

for doc in docs/dorkpipe.md src/lib/dorkpipe/README.md AGENTS.md; do
	if [[ -f "$ROOT/$doc" ]]; then
		echo "### $doc (first 40 lines)"
		head -40 "$ROOT/$doc"
		echo ""
	fi
done >"$OUT/doc_excerpts.txt" || true

echo "self-analysis-prep: wrote $OUT"
