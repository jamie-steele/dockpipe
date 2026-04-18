#!/usr/bin/env bash
# Wrapper around `dorkpipe insight review`.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/lib/dorkpipe-cli.sh"
dorkpipe_script_bootstrap "$SCRIPT_DIR"
ROOT="$(dockpipe_sdk workdir)"

dorkpipe_script_exec_cli "$SCRIPT_DIR" insight review --workdir "$ROOT" "$@"
