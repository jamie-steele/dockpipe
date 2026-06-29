#!/usr/bin/env bash
# Used by DockPipe workflow dorkpipe-self-analysis-stack — idempotent compose up.
set -euo pipefail
SCRIPT_DIR="$(dockpipe get script_dir)"
exec bash "$SCRIPT_DIR/dev-stack.sh" up
