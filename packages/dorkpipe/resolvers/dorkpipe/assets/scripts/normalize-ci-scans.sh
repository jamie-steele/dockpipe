#!/usr/bin/env bash
# Normalize gosec + govulncheck JSON into DorkPipe CI artifact state.
# for DorkPipe downstream reasoning.
# Thin wrapper around `dorkpipe ci normalize-scans`.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/lib/dorkpipe-cli.sh"

dorkpipe_script_exec_cli "$SCRIPT_DIR" ci normalize-scans
