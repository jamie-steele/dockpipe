#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
[[ -f "${DORKPIPE_ORCH_GRAPH_JSON}" ]] || {
  echo "missing task graph: ${DORKPIPE_ORCH_GRAPH_JSON}" >&2
  exit 1
}

"$(dorkpipe_orchestrate_helper_bin)" run-tasks "${DORKPIPE_ORCH_GRAPH_JSON}" "${SCRIPT_DIR}/orchestrate-run-task.sh"
