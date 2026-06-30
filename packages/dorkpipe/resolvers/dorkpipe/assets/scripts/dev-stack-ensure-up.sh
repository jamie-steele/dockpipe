#!/usr/bin/env bash
# Used by DockPipe workflows that need the DorkPipe stack — succeeds only when
# the requested services actually start and the selected GPU policy is satisfied.
set -euo pipefail
SCRIPT_DIR="$(dockpipe get script_dir)"
exec bash "$SCRIPT_DIR/dev-stack.sh" up
