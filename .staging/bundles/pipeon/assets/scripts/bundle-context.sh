#!/usr/bin/env bash
# Build .dockpipe/pipeon-context.md from repo signals (CI, insights, self-analysis pointers).
# Bounded size; no network. Run after enabling Pipeon (see lib/enable.sh).
set -euo pipefail

ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
OUT="$ROOT/.dockpipe"
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
	echo "| Repo / analysis facts | e.g. \`.dorkpipe/self-analysis/\` |"
	echo "| Scan signals | e.g. \`.dockpipe/ci-analysis/findings.json\` |"
	echo "| User guidance | e.g. \`.dockpipe/analysis/insights.json\` (signal, not truth) |"
	echo ""
	echo "## CI / scans (\`.dockpipe/ci-analysis/\`)"
	echo ""

	FIND="$ROOT/.dockpipe/ci-analysis/findings.json"
	if [[ -f "$FIND" ]] && have_jq; then
		FC="$(jq '.findings | length' "$FIND" 2>/dev/null || echo 0)"
		SC="$(jq -r '.schema_version // "?"' "$FIND")"
		FCOMMIT="$(jq -r '.provenance.commit // "unknown"' "$FIND")"
		echo "- **findings.json:** present — schema $SC, **$FC** findings, provenance commit \`$FCOMMIT\`"
		if [[ "$FCOMMIT" != "unknown" && "$FCOMMIT" != "$COMMIT" ]]; then
			echo "- **Staleness:** findings commit differs from current HEAD — refresh recommended for scan-aligned answers."
		fi
		if [[ -f "$ROOT/.dockpipe/ci-analysis/SUMMARY.md" ]]; then
			echo ""
			echo "### SUMMARY.md (excerpt)"
			echo ""
			head -25 "$ROOT/.dockpipe/ci-analysis/SUMMARY.md" | sed 's/^/    /'
		fi
	else
		echo "- **findings.json:** not present — run \`make ci\` (dockpipe repo) or CI to generate (see docs/dorkpipe-ci-signals.md)."
	fi

	echo ""
	echo "## User insights (\`.dockpipe/analysis/\`)"
	echo ""
	INS="$ROOT/.dockpipe/analysis/insights.json"
	if [[ -f "$INS" ]] && have_jq; then
		echo "- **file:** present"
		jq -r '"- count: " + ((.insights // []) | length | tostring)' "$INS" 2>/dev/null || true
		jq -r '.insights[]? | "- [" + .status + "] " + .category + ": " + (.normalized_text | .[0:120])' "$INS" 2>/dev/null | head -40
	else
		echo "- **insights.json:** not present — optional; see docs/user-insight-queue.md"
	fi

	echo ""
	echo "## Orchestrator / run metadata (\`.dorkpipe/\`)"
	echo ""
	if [[ -f "$ROOT/.dorkpipe/run.json" ]] && have_jq; then
		jq '{name, ts, policy}' "$ROOT/.dorkpipe/run.json" 2>/dev/null | sed 's/^/    /' || true
	else
		echo "- **run.json:** not present"
	fi

	echo ""
	echo "## Self-analysis bundle (\`.dorkpipe/self-analysis/\`)"
	echo ""
	if [[ -d "$ROOT/.dorkpipe/self-analysis" ]] && [[ -n "$(ls -A "$ROOT/.dorkpipe/self-analysis" 2>/dev/null)" ]]; then
		ls -1 "$ROOT/.dorkpipe/self-analysis" 2>/dev/null | head -30 | sed 's/^/- /'
	else
		echo "- Directory missing or empty — run maintainer self-analysis workflows if you use DorkPipe signals."
	fi

	echo ""
	echo "## Pointers"
	echo ""
	echo "- **AGENTS.md** — maintainer/agent contract for this repo."
	echo "- **docs/compliance-ai-handoff.md** — how to discuss compliance without claiming certification."
	echo "- **src/apps/pipeon/docs/pipeon-ide-experience.md** — Pipeon UX and tone."
	echo ""
} >"$CTX"

echo "pipeon: wrote $CTX"
