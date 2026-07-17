#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_plan_main() {
  local followup_mode source_workflow_config source_step_id helper_bin
  followup_mode="0"
  if [[ -n "${DORKPIPE_ORCH_FOLLOWUP_REQUEST:-}" || -n "${DORKPIPE_ORCH_FOLLOWUP_GOAL:-}" || -n "${DORKPIPE_ORCH_FOLLOWUP_TASK_IDS:-}" ]]; then
    followup_mode="1"
  fi
  if [[ "${followup_mode}" != "1" ]]; then
    find "${DORKPIPE_ORCH_TASKS_DIR}" -mindepth 1 -maxdepth 1 -exec rm -rf {} + || return 1
    find "${DORKPIPE_ORCH_SHARED_DIR}" -mindepth 1 -maxdepth 1 -exec rm -rf {} + || return 1
    find "${DORKPIPE_ORCH_LANES_DIR}" -mindepth 1 -maxdepth 1 -exec rm -rf {} + || return 1
  fi
  rm -f "${DORKPIPE_ORCH_HALT_JSON}" || return 1
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
  source_workflow_config="${DORKPIPE_ORCH_SOURCE_WORKFLOW_CONFIG:-${DOCKPIPE_WORKFLOW_CONFIG:-}}"
  source_step_id="${DORKPIPE_ORCH_SOURCE_STEP_ID:-${DOCKPIPE_STEP_ID:-}}"

  [[ -n "${source_workflow_config}" ]] || {
    echo "DORKPIPE_ORCH_SOURCE_WORKFLOW_CONFIG or DOCKPIPE_WORKFLOW_CONFIG is required" >&2
    return 1
  }
  [[ -f "${source_workflow_config}" ]] || {
    echo "missing workflow config: ${source_workflow_config}" >&2
    return 1
  }
  [[ -n "${source_step_id}" ]] || {
    echo "DORKPIPE_ORCH_SOURCE_STEP_ID or DOCKPIPE_STEP_ID is required for orchestration planning" >&2
    return 1
  }

  helper_bin="$(dorkpipe_orchestrate_helper_bin)" || return 1
  "${helper_bin}" plan "${source_workflow_config}" "${source_step_id}" || return 1
}

dorkpipe_orchestrate_init
started_ms="$(dorkpipe_orchestrate_now_ms)"
plan_followup="false"
if [[ -n "${DORKPIPE_ORCH_FOLLOWUP_REQUEST:-}" || -n "${DORKPIPE_ORCH_FOLLOWUP_GOAL:-}" || -n "${DORKPIPE_ORCH_FOLLOWUP_TASK_IDS:-}" ]]; then
  plan_followup="true"
fi
dorkpipe_orchestrate_operation_emit "orchestrate.plan" "start" "" "workflow=${DORKPIPE_ORCH_WORKFLOW:-}" "root=${DORKPIPE_ORCH_ROOT:-}" "followup=${plan_followup}"
if ! dorkpipe_orchestrate_plan_main; then
  dorkpipe_orchestrate_operation_fail "orchestrate.plan" "${started_ms}" "orchestration planning failed" "workflow=${DORKPIPE_ORCH_WORKFLOW:-}" "root=${DORKPIPE_ORCH_ROOT:-}"
  exit 1
fi
duration_ms="$(dorkpipe_orchestrate_operation_duration_ms "${started_ms}")"
dorkpipe_orchestrate_operation_emit "orchestrate.plan" "done" "${duration_ms}" "workflow=${DORKPIPE_ORCH_WORKFLOW:-}" "root=${DORKPIPE_ORCH_ROOT:-}" "followup=${plan_followup}" "plan=${DORKPIPE_ORCH_LANES_DIR}/plan.json"
