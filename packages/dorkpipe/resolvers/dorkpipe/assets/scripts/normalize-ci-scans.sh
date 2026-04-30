#!/usr/bin/env bash
# Normalize gosec + govulncheck JSON into bin/.dockpipe/ci-analysis/ for DorkPipe downstream reasoning.
# Thin wrapper around `dorkpipe ci normalize-scans`.
set -euo pipefail

SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:?DOCKPIPE_SCRIPT_DIR is required}"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/lib/dorkpipe-cli.sh"
ROOT="${DOCKPIPE_WORKDIR:?DOCKPIPE_WORKDIR is required}"

dorkpipe_script_exec_cli "$SCRIPT_DIR" ci normalize-scans --workdir "$ROOT"
