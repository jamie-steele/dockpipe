#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
find "${DORKPIPE_ORCH_TASKS_DIR}" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
find "${DORKPIPE_ORCH_SHARED_DIR}" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
find "${DORKPIPE_ORCH_LANES_DIR}" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
rm -f "${DORKPIPE_ORCH_HALT_JSON}"
cat > "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" <<EOF
{
  "max_total_cloud_tokens": ${DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS},
  "max_task_cloud_tokens": ${DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS},
  "stop_on_budget_exceeded": ${DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED},
  "total_estimated_input_tokens": 0,
  "total_estimated_output_tokens": 0,
  "total_estimated_tokens": 0,
  "total_duration_ms": 0,
  "cloud_task_count": 0,
  "budget_exceeded": false,
  "halted": false,
  "providers": {
    "codex": {"task_count": 0, "estimated_tokens": 0, "duration_ms": 0},
    "claude": {"task_count": 0, "estimated_tokens": 0, "duration_ms": 0}
  }
}
EOF
[[ -n "${DOCKPIPE_WORKFLOW_CONFIG:-}" ]] || {
  echo "DOCKPIPE_WORKFLOW_CONFIG is required" >&2
  exit 1
}
[[ -f "${DOCKPIPE_WORKFLOW_CONFIG}" ]] || {
  echo "missing workflow config: ${DOCKPIPE_WORKFLOW_CONFIG}" >&2
  exit 1
}
[[ -n "${DOCKPIPE_STEP_ID:-}" ]] || {
  echo "DOCKPIPE_STEP_ID is required for orchestration planning" >&2
  exit 1
}

"$(dorkpipe_orchestrate_helper_bin)" plan "${DOCKPIPE_WORKFLOW_CONFIG}" "${DOCKPIPE_STEP_ID}"

printf '[dorkpipe] orchestration plan ready at %s\n' "${DORKPIPE_ORCH_ROOT}" >&2
