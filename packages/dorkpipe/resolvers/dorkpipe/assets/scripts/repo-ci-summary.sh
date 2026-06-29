#!/usr/bin/env bash
# Host-only: print DorkPipe-normalized CI scans after govulncheck + gosec + normalize-ci-scans
# (same spirit as the Linux CI job). Run: dockpipe --workflow dockpipe-repo-quality --workdir . --
set -euo pipefail

CI_ANALYSIS_DIR="$(dockpipe scope artifacts ci-analysis)"
if [[ -d "$CI_ANALYSIS_DIR" ]]; then
	echo "=== DockPipe CI analysis ($CI_ANALYSIS_DIR/) ==="
	find "$CI_ANALYSIS_DIR" -type f | sort | head -50
	echo ""
	echo "Populated by: govulncheck + gosec + dorkpipe ci normalize-scans (wrapper: packages/dorkpipe/resolvers/dorkpipe/assets/scripts/normalize-ci-scans.sh; see ci-local.sh / CI)."
else
	echo "No $CI_ANALYSIS_DIR/ yet."
	echo "Run:  bash src/scripts/ci-local.sh"
	echo "  (or the govulncheck + gosec + normalize steps from .github/workflows/ci.yml), then re-run this workflow."
fi
