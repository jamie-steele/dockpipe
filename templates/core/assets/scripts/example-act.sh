#!/usr/bin/env bash
# Example act script (runs after command in container). Use as a starting point.
# Config points to scripts/example-act.sh. Reads: DOCKPIPE_EXIT_CODE, DOCKPIPE_CONTAINER_WORKDIR.
set -euo pipefail

echo "[example-act] Exit code: ${DOCKPIPE_EXIT_CODE:-0}" >&2
echo "[example-act] Edit this script or add scripts from dockpipe's scripts/." >&2
exit "${DOCKPIPE_EXIT_CODE:-0}"
