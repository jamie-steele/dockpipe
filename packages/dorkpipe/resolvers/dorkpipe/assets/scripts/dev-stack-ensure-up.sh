#!/usr/bin/env bash
# Used by DockPipe workflow dorkpipe-self-analysis-stack — idempotent compose up.
set -euo pipefail
SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:?DOCKPIPE_SCRIPT_DIR is required}"
exec "$SCRIPT_DIR/dev-stack.sh" up
