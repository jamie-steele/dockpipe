#!/usr/bin/env bash
# Host-only: print DorkPipe-normalized CI scans after govulncheck + gosec + normalize-ci-scans
# (same spirit as the Linux CI job). Run: dockpipe --workflow dockpipe-repo-quality --workdir . --
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/lib/dorkpipe-cli.sh"
ROOT="$(dockpipe get workdir)"
if [[ -n "${DOCKPIPE_SDK_SH:-}" && -f "$DOCKPIPE_SDK_SH" ]]; then
	# shellcheck source=/dev/null
	source "$DOCKPIPE_SDK_SH"
	dockpipe_sdk_refresh "$ROOT"
else
	eval "$(dockpipe sdk --workdir "$ROOT")"
fi

cd "$ROOT"
CI_ANALYSIS_DIR="$(dockpipe_sdk ci analysis)"
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
