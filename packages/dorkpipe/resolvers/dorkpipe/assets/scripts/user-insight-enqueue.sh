#!/usr/bin/env bash
# Wrapper around `dorkpipe insight enqueue`.
set -euo pipefail

SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:?DOCKPIPE_SCRIPT_DIR is required}"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/lib/dorkpipe-cli.sh"
ROOT="${DOCKPIPE_WORKDIR:?DOCKPIPE_WORKDIR is required}"

dorkpipe_script_exec_cli "$SCRIPT_DIR" insight enqueue --workdir "$ROOT" "$@"
