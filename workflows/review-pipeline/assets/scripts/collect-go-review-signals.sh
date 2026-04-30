#!/usr/bin/env bash
# Deterministic prep: Go file list + bounded pattern hits for downstream review bundles.
# Writes under bin/.dockpipe/ — reusable from any workflow; requires bash + git or find + grep.
set -euo pipefail

ROOT="$(dockpipe get workdir)"
cd "$ROOT"
OUT="${ROOT}/bin/.dockpipe"
mkdir -p "$OUT"

FILES="$OUT/review-files.txt"
SIGS="$OUT/review-signals.txt"
: >"$FILES"
: >"$SIGS"

if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
	git ls-files '*.go' 2>/dev/null | head -n 500 >>"$FILES" || true
fi
if [[ ! -s "$FILES" ]]; then
	find . -name '*.go' -not -path './.git/*' -not -path './bin/.dockpipe/*' -not -path './.dockpipe/*' 2>/dev/null | head -n 500 | sed 's|^\./||' >>"$FILES" || true
fi

{
	echo "## Bounded pattern hits (grep, capped per pattern)"
	# shellcheck disable=SC2046
	for pat in 'exec\.Command' 'os/exec' 'ioutil\.' 'Deprecated:' 'TODO' 'FIXME' 'unsafe\.'; do
		echo "### $pat"
		grep -R -n --include='*.go' --exclude-dir=.git --exclude-dir=vendor -E "$pat" . 2>/dev/null | head -n 25 || true
		echo ""
	done
} >>"$SIGS"

NFILES=$(wc -l <"$FILES" | tr -d ' ')
NSIGS=$(wc -l <"$SIGS" | tr -d ' ')
echo "$NFILES" >"$OUT/review-files.count"
echo "$NSIGS" >"$OUT/review-signals.lines"
echo "[review] collect-go-review-signals: ${NFILES} go files, ${NSIGS} signal lines (bounded)" >&2
