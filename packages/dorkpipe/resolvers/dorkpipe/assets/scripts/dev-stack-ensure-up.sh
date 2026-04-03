#!/usr/bin/env bash
# Used by DockPipe workflow dorkpipe-self-analysis-stack — idempotent compose up.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$SCRIPT_DIR/dev-stack.sh" up
