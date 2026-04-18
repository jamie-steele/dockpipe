#!/usr/bin/env bash
# Normalize gosec + govulncheck JSON into bin/.dockpipe/ci-analysis/ for DorkPipe downstream reasoning.
# Thin wrapper around `dorkpipe ci normalize-scans`.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../../../.." && pwd)"
export DOCKPIPE_WORKDIR="${DOCKPIPE_WORKDIR:-$(pwd)}"
# shellcheck source=/dev/null
source "$REPO_ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh"
ROOT="$(dockpipe_sdk workdir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/lib/dorkpipe-cli.sh"

dorkpipe_script_exec_cli "$SCRIPT_DIR" ci normalize-scans --workdir "$ROOT"
