#!/usr/bin/env bash
# Wrapper around `dorkpipe insight supersede`.
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/lib/dorkpipe-cli.sh"
ROOT="$(dockpipe get workdir)"

dorkpipe_script_exec_cli "$SCRIPT_DIR" insight supersede --workdir "$ROOT" "$@"
