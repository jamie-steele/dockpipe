#!/usr/bin/env bash
# Wrapper around `dorkpipe handoff build-cursor`.
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/lib/dorkpipe-cli.sh"

dorkpipe_script_exec_cli "$SCRIPT_DIR" handoff build-cursor "$@"
