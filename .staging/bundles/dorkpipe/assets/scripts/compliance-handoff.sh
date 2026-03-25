#!/usr/bin/env bash
# Host-side handoff for AI: compliance / security posture — loads artifact paths only (no scoring).
# Framework: DockPipe runs this script; DorkPipe / assistants interpret docs/compliance-ai-handoff.md
set -euo pipefail

ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"

echo ""
echo "=== DockPipe — compliance & security posture handoff (signals only) ==="
echo "Read: docs/compliance-ai-handoff.md"
echo ""

have_jq() { command -v jq >/dev/null 2>&1; }

if [[ -f "$ROOT/.dockpipe/ci-analysis/findings.json" ]]; then
	echo "--- .dockpipe/ci-analysis/ (CI-normalized signals) ---"
	if have_jq; then
		jq -r '"schema: " + .schema_version + " | findings: " + (.findings|length|tostring) + " | commit: " + .provenance.commit + " | source: " + .provenance.source' "$ROOT/.dockpipe/ci-analysis/findings.json"
	else
		echo "(install jq for JSON summary)"
		ls -la "$ROOT/.dockpipe/ci-analysis/"
	fi
else
	echo "[ ] .dockpipe/ci-analysis/findings.json — run: bash scripts/ci-local.sh (or CI) to generate"
fi

if [[ -f "$ROOT/.dockpipe/ci-analysis/SUMMARY.md" ]]; then
	echo ""
	echo "--- SUMMARY.md (head) ---"
	head -15 "$ROOT/.dockpipe/ci-analysis/SUMMARY.md"
fi

if [[ -d "$ROOT/.dorkpipe/self-analysis" ]] && [[ -n "$(ls -A "$ROOT/.dorkpipe/self-analysis" 2>/dev/null)" ]]; then
	echo ""
	echo "--- .dorkpipe/self-analysis/ (present) ---"
	ls -la "$ROOT/.dorkpipe/self-analysis" | head -20
fi

if [[ -f "$ROOT/.dorkpipe/run.json" ]]; then
	echo ""
	echo "--- .dorkpipe/run.json ---"
	if have_jq; then
		jq '{name, ts, policy}' "$ROOT/.dorkpipe/run.json" 2>/dev/null || true
	else
		head -5 "$ROOT/.dorkpipe/run.json"
	fi
fi

if [[ -f "$ROOT/.dockpipe/analysis/insights.json" ]]; then
	echo ""
	echo "--- .dockpipe/analysis/insights.json (user guidance signals; not verified facts) ---"
	if have_jq; then
		jq '{kind, insight_count: (.insights | length), categories: [.insights[].category] | unique}' "$ROOT/.dockpipe/analysis/insights.json" 2>/dev/null || true
	else
		head -8 "$ROOT/.dockpipe/analysis/insights.json"
	fi
fi

echo ""
echo "AI: Answer compliance/security questions using AGENTS.md + artifacts above; do not claim certified compliance."
echo "See docs/compliance-ai-handoff.md"
