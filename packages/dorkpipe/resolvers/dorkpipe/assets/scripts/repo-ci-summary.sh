#!/usr/bin/env bash
# Host-only: print what is in bin/.dockpipe/ci-analysis/ after govulncheck + gosec + normalize-ci-scans
# (same spirit as the Linux CI job). Run: dockpipe --workflow dockpipe-repo-quality --workdir . --
set -euo pipefail

SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:?DOCKPIPE_SCRIPT_DIR is required}"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/lib/dorkpipe-cli.sh"
ROOT="${DOCKPIPE_WORKDIR:?DOCKPIPE_WORKDIR is required}"

cd "$ROOT"
if [[ -d bin/.dockpipe/ci-analysis ]]; then
	echo "=== DockPipe CI analysis (bin/.dockpipe/ci-analysis/) ==="
	find bin/.dockpipe/ci-analysis -type f | sort | head -50
	echo ""
	echo "Populated by: govulncheck + gosec + dorkpipe ci normalize-scans (wrapper: packages/dorkpipe/resolvers/dorkpipe/assets/scripts/normalize-ci-scans.sh; see ci-local.sh / CI)."
else
	echo "No bin/.dockpipe/ci-analysis/ yet."
	echo "Run:  bash src/scripts/ci-local.sh"
	echo "  (or the govulncheck + gosec + normalize steps from .github/workflows/ci.yml), then re-run this workflow."
fi
