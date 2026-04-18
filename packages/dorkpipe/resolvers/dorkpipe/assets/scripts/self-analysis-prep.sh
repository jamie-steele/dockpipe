#!/usr/bin/env bash
# Deterministic repo facts for DorkPipe self-analysis (no LLM). Writes bin/.dockpipe/packages/dorkpipe/self-analysis/
set -euo pipefail
ROOT="${DOCKPIPE_WORKDIR:?DOCKPIPE_WORKDIR is required}"
OUT="$ROOT/bin/.dockpipe/packages/dorkpipe/self-analysis"
mkdir -p "$OUT"

{
	echo "## git"
	git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo "(not a git repo)"
	git -C "$ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || true
	git -C "$ROOT" status -sb 2>/dev/null | head -20 || true
} >"$OUT/git.txt"

if [[ -d "$ROOT/packages/dorkpipe/lib" ]]; then
	find "$ROOT/packages/dorkpipe/lib" -mindepth 1 -maxdepth 1 -type d | sort | while read -r d; do
		n=$(find "$d" -name '*.go' 2>/dev/null | wc -l | tr -d ' ')
		printf '%s\t%s\n' "$(basename "$d")" "$n"
	done >"$OUT/dorkpipe_packages.tsv"
else
	: >"$OUT/dorkpipe_packages.tsv"
fi

find "$ROOT/packages/dorkpipe/lib" -name '*.go' 2>/dev/null | wc -l | tr -d ' ' >"$OUT/dorkpipe_go_files.count" || echo 0 >"$OUT/dorkpipe_go_files.count"

{
	echo "## cmd"
	find "$ROOT/src/cmd" -name '*.go' 2>/dev/null | head -40
} >"$OUT/cmd_go_files.txt" || true

{
	echo "## dorkpipe resolver assets/scripts (package)"
	ls -la "$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts" 2>/dev/null || true
} >"$OUT/scripts_dorkpipe_ls.txt"

{
	echo "## packages/dorkpipe (sample)"
	find "$ROOT/packages/dorkpipe" -type f 2>/dev/null | sort | head -60
} >"$OUT/assets_dorkpipe_files.txt" || true

{
	echo "## key file line counts"
	for f in \
		packages/dorkpipe/lib/engine/run.go \
		packages/dorkpipe/lib/spec/spec.go \
		packages/dorkpipe/lib/aggregator/merge.go \
		packages/dorkpipe/lib/workers/workers.go \
		packages/dorkpipe/lib/cmd/dorkpipe/main.go; do
		if [[ -f "$ROOT/$f" ]]; then
			wc -l "$ROOT/$f"
		fi
	done
} >"$OUT/key_file_wc.txt" || true

{
	echo "## workflows/"
	find "$ROOT/workflows" -name 'config.yml' 2>/dev/null | sort
} >"$OUT/workflow_configs.txt" || true

for doc in packages/dorkpipe/lib/README.md AGENTS.md docs/artifacts.md; do
	if [[ -f "$ROOT/$doc" ]]; then
		echo "### $doc (first 40 lines)"
		head -40 "$ROOT/$doc"
		echo ""
	fi
done >"$OUT/doc_excerpts.txt" || true

echo "self-analysis-prep: wrote $OUT"
