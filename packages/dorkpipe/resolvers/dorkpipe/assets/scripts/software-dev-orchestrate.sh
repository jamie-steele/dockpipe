#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

software_dev_planner_enabled() {
  dorkpipe_orchestrate_bool "${DORKPIPE_SOFTWARE_DEV_PLANNER_MODE:-false}"
}

software_dev_require_selection() {
  [[ -n "${DORKPIPE_SOFTWARE_DEV_TASK_PACK:-}" ]] || {
    echo "DORKPIPE_SOFTWARE_DEV_TASK_PACK is required" >&2
    return 1
  }
  [[ -n "${DORKPIPE_SOFTWARE_DEV_TASK_PACK_STEP:-}" ]] || {
    echo "DORKPIPE_SOFTWARE_DEV_TASK_PACK_STEP is required" >&2
    return 1
  }
}

software_dev_init_usage() {
  cat >"${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" <<EOF
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
}

software_dev_cap_budget() {
  local current="${1:-}" ceiling="${2:?ceiling}"
  if [[ ! "${current}" =~ ^[0-9]+$ ]] || (( current > ceiling )); then
    printf '%s\n' "${ceiling}"
  else
    printf '%s\n' "${current}"
  fi
}

dorkpipe_orchestrate_init
if command -v cygpath >/dev/null 2>&1; then
  for path_var in ROOT DOCKPIPE_WORKFLOW_CONFIG DORKPIPE_ORCH_ROOT DORKPIPE_ORCH_TASKS_DIR DORKPIPE_ORCH_SHARED_DIR DORKPIPE_ORCH_GRAPH_JSON DORKPIPE_ORCH_CLOUD_USAGE_JSON DORKPIPE_ORCH_HALT_JSON; do
    path_value="${!path_var:-}"
    if [[ -n "${path_value}" ]]; then
      printf -v "${path_var}" '%s' "$(cygpath -m "${path_value}")"
      export "${path_var}"
    fi
  done
fi
export DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS="$(software_dev_cap_budget "${DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS:-}" 60000)"
export DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS="$(software_dev_cap_budget "${DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS:-}" 20000)"
step_id="${DOCKPIPE_STEP_ID:-}"
helper_bin="$(dorkpipe_orchestrate_helper_bin)"

case "${step_id}" in
  planner_contract|contract)
    exit 0
    ;;
  prepare)
    software_dev_require_selection
    software_dev_init_usage
    rm -f "${DORKPIPE_ORCH_HALT_JSON}"
    if [[ "$(software_dev_planner_enabled)" == "true" ]]; then
      find "${DORKPIPE_ORCH_TASKS_DIR}" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
      find "${DORKPIPE_ORCH_SHARED_DIR}" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
      MSYS2_ARG_CONV_EXCL='*' "${helper_bin}" software-dev-stage-proposal \
        "${ROOT}" \
        "${DORKPIPE_SOFTWARE_DEV_TASK_PACK}" \
        "${DORKPIPE_ORCH_SHARED_DIR}/software-dev-task-pack.yml"
      MSYS2_ARG_CONV_EXCL='*' "${helper_bin}" plan "${DOCKPIPE_WORKFLOW_CONFIG}" planner_contract
    else
      MSYS2_ARG_CONV_EXCL='*' "${helper_bin}" software-dev-compile \
        "${DOCKPIPE_WORKFLOW_CONFIG}" contract \
        "${ROOT}" \
        "${DORKPIPE_SOFTWARE_DEV_TASK_PACK}" \
        "${DORKPIPE_SOFTWARE_DEV_TASK_PACK_STEP}" \
        "${DORKPIPE_ORCH_ROOT}"
    fi
    ;;
  planner_auth)
    if [[ "$(software_dev_planner_enabled)" == "true" ]]; then
      bash "${SCRIPT_DIR}/orchestrate-auth-preflight.sh"
    fi
    ;;
  planner_run)
    if [[ "$(software_dev_planner_enabled)" != "true" ]]; then
      exit 0
    fi
    if [[ -n "${DORKPIPE_SOFTWARE_DEV_PLANNER_PROPOSAL_FIXTURE:-}" ]]; then
      MSYS2_ARG_CONV_EXCL='*' "${helper_bin}" software-dev-stage-proposal \
        "${ROOT}" \
        "${DORKPIPE_SOFTWARE_DEV_PLANNER_PROPOSAL_FIXTURE}" \
        "${DORKPIPE_ORCH_TASKS_DIR}/software_dev_planner/response.md"
    else
      MSYS2_ARG_CONV_EXCL='*' "${helper_bin}" run-tasks "${DORKPIPE_ORCH_GRAPH_JSON}" "${SCRIPT_DIR}/orchestrate-run-task.sh"
    fi
    ;;
  compile)
    if [[ "$(software_dev_planner_enabled)" != "true" ]]; then
      exit 0
    fi
    MSYS2_ARG_CONV_EXCL='*' "${helper_bin}" software-dev-compile \
      "${DOCKPIPE_WORKFLOW_CONFIG}" contract \
      "${ROOT}" \
      "${DORKPIPE_SOFTWARE_DEV_TASK_PACK}" \
      "${DORKPIPE_SOFTWARE_DEV_TASK_PACK_STEP}" \
      "${DORKPIPE_ORCH_ROOT}" \
      "${DORKPIPE_ORCH_TASKS_DIR}/software_dev_planner/response.md"
    ;;
  worker_auth)
    bash "${SCRIPT_DIR}/orchestrate-auth-preflight.sh"
    ;;
  *)
    echo "unsupported software.dev workflow step: ${step_id:-<empty>}" >&2
    exit 1
    ;;
esac
