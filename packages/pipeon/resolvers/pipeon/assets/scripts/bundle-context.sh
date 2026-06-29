#!/usr/bin/env bash
# Build the package-scoped Pipeon context bundle from repo signals (CI, insights, self-analysis pointers).
# Bounded size; no network. Run after enabling Pipeon (see lib/enable.sh).
set -euo pipefail

ROOT="$(dockpipe get workdir)"
if [[ -n "${DOCKPIPE_SDK_SH:-}" && -f "$DOCKPIPE_SDK_SH" ]]; then
	# shellcheck source=/dev/null
	source "$DOCKPIPE_SDK_SH"
	dockpipe_sdk_refresh "$ROOT"
else
	eval "$(dockpipe sdk --workdir "$ROOT")"
fi
OUT="$(dockpipe_sdk scope --package pipeon .)"
CTX="$OUT/pipeon-context.md"
mkdir -p "$OUT"

TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
COMMIT="$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
BRANCH="$(git -C "$ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
VER=""
[[ -f "$ROOT/VERSION" ]] && VER="$(tr -d ' \t\r\n' <"$ROOT/VERSION")"

have_jq() { command -v jq >/dev/null 2>&1; }

{
	echo "# Pipeon context bundle"
	echo ""
	echo "- **Generated (UTC):** $TS"
	echo "- **Repo VERSION:** ${VER:-unknown}"
	echo "- **git HEAD:** \`$COMMIT\` branch \`$BRANCH\`"
	echo ""
	echo "## Separation (do not merge lanes)"
	echo ""
	echo "| Lane | Meaning |"
	echo "|------|---------|"
	echo "| Repo / analysis facts | DorkPipe package self-analysis artifacts |"
	echo "| Scan signals | DockPipe CI analysis scope for the active workflow |"
	echo "| User guidance | DorkPipe package-scoped analysis insights (signal, not truth) |"
	echo ""
	echo "## CI / scans"
	echo ""

	CI_ANALYSIS_DIR="$(dockpipe_sdk ci analysis)"
	echo ""
	echo "- **analysis dir:** \`$CI_ANALYSIS_DIR\`"
	FIND="$CI_ANALYSIS_DIR/findings.json"
	if [[ -f "$FIND" ]] && have_jq; then
		FC="$(jq '.findings | length' "$FIND" 2>/dev/null || echo 0)"
		SC="$(jq -r '.schema_version // "?"' "$FIND")"
		FCOMMIT="$(jq -r '.provenance.commit // "unknown"' "$FIND")"
		echo "- **findings.json:** present — schema $SC, **$FC** findings, provenance commit \`$FCOMMIT\`"
		if [[ "$FCOMMIT" != "unknown" && "$FCOMMIT" != "$COMMIT" ]]; then
			echo "- **Staleness:** findings commit differs from current HEAD — refresh recommended for scan-aligned answers."
		fi
		if [[ -f "$CI_ANALYSIS_DIR/SUMMARY.md" ]]; then
			echo ""
			echo "### SUMMARY.md (excerpt)"
			echo ""
			head -25 "$CI_ANALYSIS_DIR/SUMMARY.md" | sed 's/^/    /'
		fi
	else
		echo "- **findings.json:** not present — run \`make ci\` (dockpipe repo) or CI to generate (see docs/dorkpipe-ci-signals.md)."
	fi

	echo ""
	echo "## User insights (DockPipe analysis scope)"
	echo ""
	INS="$(dockpipe_sdk scope --package dorkpipe analysis/insights.json)"
	if [[ -f "$INS" ]] && have_jq; then
		echo "- **file:** present"
		jq -r '"- count: " + ((.insights // []) | length | tostring)' "$INS" 2>/dev/null || true
		jq -r '.insights[]? | "- [" + .status + "] " + .category + ": " + (.normalized_text | .[0:120])' "$INS" 2>/dev/null | head -40
	else
		echo "- **insights.json:** not present — optional; see docs/user-insight-queue.md"
	fi

	echo ""
	echo "## Orchestrator / run metadata (DorkPipe package scope)"
	echo ""
	DORKPIPE_STATE_DIR="$(dockpipe_sdk scope --package dorkpipe .)"
	if [[ -f "$DORKPIPE_STATE_DIR/run.json" ]] && have_jq; then
		jq '{name, ts, policy}' "$DORKPIPE_STATE_DIR/run.json" 2>/dev/null | sed 's/^/    /' || true
	else
		echo "- **run.json:** not present"
	fi

	echo ""
	echo "## Self-analysis bundle (DorkPipe package scope)"
	echo ""
	DORKPIPE_SELF_ANALYSIS_DIR="$DORKPIPE_STATE_DIR/self-analysis"
	if [[ -d "$DORKPIPE_SELF_ANALYSIS_DIR" ]] && [[ -n "$(ls -A "$DORKPIPE_SELF_ANALYSIS_DIR" 2>/dev/null)" ]]; then
		ls -1 "$DORKPIPE_SELF_ANALYSIS_DIR" 2>/dev/null | head -30 | sed 's/^/- /'
	else
		echo "- Directory missing or empty — run maintainer self-analysis workflows if you use DorkPipe signals."
	fi

	echo ""
	echo "## Pointers"
	echo ""
	echo "- **AGENTS.md** — maintainer/agent contract for this repo."
	echo "- **docs/compliance-ai-handoff.md** — how to discuss compliance without claiming certification."
	echo "- **Pipeon UX** — see assets/docs/pipeon-ide-experience.md (pipeon resolver)."
	echo ""
} >"$CTX"

echo "pipeon: wrote $CTX"
