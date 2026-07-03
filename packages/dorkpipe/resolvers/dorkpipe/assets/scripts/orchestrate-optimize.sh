#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init

action="${DORKPIPE_OPTIMIZER_ACTION:-prepare}"
target_workflow="${DORKPIPE_OPTIMIZER_TARGET_WORKFLOW:-docs.orchestrate}"
target_root="$(dockpipe scope workflow "${target_workflow}" orchestrate)"
optimizer_root="$(dockpipe scope workflow "${target_workflow}" optimize)"

resolve_path() {
  local path="${1:?path}"
  if [[ "${path}" =~ ^[A-Za-z][\\/].* ]]; then
    echo "orchestrate-optimize: invalid drive path uses U+F03A instead of ':' -> ${path}" >&2
    exit 1
  fi
  case "${path}" in
    /*|[A-Za-z]:\\*|[A-Za-z]:/*|\\\\*)
      printf '%s\n' "${path}"
      ;;
    *)
      printf '%s/%s\n' "${ROOT}" "${path}"
      ;;
  esac
}

optimizer_dir="$(resolve_path "${optimizer_root}")"
target_dir="$(resolve_path "${target_root}")"
result_json="${optimizer_dir}/${action}/result.json"

mkdir -p "${optimizer_dir}/${action}"

case "${action}" in
  iterate|prepare|assess|propose|apply|apply-if-enabled|validate) ;;
  *)
    echo "orchestrate-optimize: unknown DORKPIPE_OPTIMIZER_ACTION=${action}" >&2
    exit 1
    ;;
esac

optimizer_unit="orchestrate.optimize"
optimizer_started_ms="$(dorkpipe_orchestrate_now_ms)"
optimizer_completed="0"
dorkpipe_orchestrate_operation_emit "${optimizer_unit}" "start" "" \
  "action=${action}" \
  "target_workflow=${target_workflow}" \
  "result=${result_json}"

finish_optimizer_operation() {
  local lifecycle_status="${1:?status}"
  local result_status="${2:-}"
  shift 2 || true
  local duration_ms
  duration_ms="$(dorkpipe_orchestrate_operation_duration_ms "${optimizer_started_ms}")"
  optimizer_completed="1"
  dorkpipe_orchestrate_operation_emit "${optimizer_unit}" "${lifecycle_status}" "${duration_ms}" \
    "action=${action}" \
    "target_workflow=${target_workflow}" \
    "result=${result_json}" \
    "status=${result_status:-${lifecycle_status}}" \
    "$@"
}

optimizer_exit_trap() {
  local rc=$?
  if [[ "${optimizer_completed}" != "1" && "${rc}" -ne 0 ]]; then
    dorkpipe_orchestrate_operation_fail "${optimizer_unit}" "${optimizer_started_ms}" "optimizer action failed" \
      "action=${action}" \
      "target_workflow=${target_workflow}" \
      "result=${result_json}" \
      "exit_code=${rc}"
  fi
}
trap optimizer_exit_trap EXIT

if [[ "${action}" == "iterate" ]]; then
  iterations="${DORKPIPE_OPTIMIZER_ITERATIONS:-1}"
  child_package="${DORKPIPE_OPTIMIZER_CHILD_PACKAGE:-}"
  child_workflow="${DORKPIPE_OPTIMIZER_CHILD_WORKFLOW:-docs.optimize-orchestrate}"
  target_package="${DORKPIPE_OPTIMIZER_TARGET_PACKAGE:-}"
  iteration_root="$(dockpipe scope workflow "${target_workflow}" optimize iterations)"
  stop_on_invalid_patch="${DORKPIPE_OPTIMIZER_STOP_ON_INVALID_PATCH:-1}"
  refresh_target_after_apply="${DORKPIPE_OPTIMIZER_REFRESH_TARGET_AFTER_APPLY:-0}"
  case "${iterations}" in
    ''|*[!0-9]*)
      echo "orchestrate-optimize: DORKPIPE_OPTIMIZER_ITERATIONS must be a positive integer" >&2
      exit 1
      ;;
  esac
  if (( iterations < 1 || iterations > 50 )); then
    echo "orchestrate-optimize: DORKPIPE_OPTIMIZER_ITERATIONS must be between 1 and 50" >&2
    exit 1
  fi

  if (( iterations == 1 )); then
    cat > "${result_json}" <<EOF
{
  "status": "skipped",
  "reason": "single optimizer pass",
  "iterations": 1
}
EOF
    finish_optimizer_operation "done" "skipped" "reason=single_optimizer_pass" "iterations=1"
    exit 0
  fi

  run_id="$(date -u +%Y%m%dT%H%M%SZ)"
  run_dir="$(resolve_path "${iteration_root}")/${run_id}"
  mkdir -p "${run_dir}"
  cat > "${run_dir}/summary.md" <<EOF
# Optimizer Iteration Run

- Child workflow: \`${child_workflow}\`
- Iterations requested: ${iterations}
- Apply enabled: ${DORKPIPE_OPTIMIZER_APPLY:-0}

EOF

  child_args=()
  if [[ -n "${child_package}" ]]; then
    child_args+=(--package "${child_package}")
  fi
  child_args+=(--workflow "${child_workflow}")

  target_args=()
  if [[ -n "${target_package}" ]]; then
    target_args+=(--package "${target_package}")
  fi
  target_args+=(--workflow "${target_workflow}")

  for i in $(seq 1 "$((iterations - 1))"); do
    iteration_started_ms="$(dorkpipe_orchestrate_now_ms)"
    dorkpipe_orchestrate_operation_emit "orchestrate.optimize.iteration" "start" "" \
      "iteration=${i}" \
      "iterations=${iterations}" \
      "target_workflow=${target_workflow}" \
      "child_workflow=${child_workflow}"
    DORKPIPE_OPTIMIZER_ITERATIONS=1 \
      DORKPIPE_OPTIMIZER_ITERATION="${i}" \
      dockpipe "${child_args[@]}" --

    iter_dir="${run_dir}/iter-${i}"
    mkdir -p "${iter_dir}"
    if [[ -d "${optimizer_dir}" ]]; then
      mkdir -p "${iter_dir}/optimize"
      while IFS= read -r -d '' optimizer_item; do
        if [[ "$(basename "${optimizer_item}")" == "iterations" ]]; then
          continue
        fi
        cp -a "${optimizer_item}" "${iter_dir}/optimize/"
      done < <(find "${optimizer_dir}" -mindepth 1 -maxdepth 1 -print0)
    fi
    orch_dir="$(resolve_path "${DORKPIPE_ORCH_ROOT}")"
    if [[ -d "${orch_dir}" ]]; then
      cp -a "${orch_dir}" "${iter_dir}/orchestrate"
    fi
    git -C "${ROOT}" status --short > "${iter_dir}/git-status.txt" || true
    printf -- '- iter-%02d: `%s`\n' "${i}" "${iter_dir}" >> "${run_dir}/summary.md"

    apply_status="$("$(dorkpipe_orchestrate_helper_bin)" optimizer-result-status "${optimizer_dir}/apply-if-enabled/result.json" 2>/dev/null || true)"
    propose_invalid="$("$(dorkpipe_orchestrate_helper_bin)" optimizer-propose-invalid "${optimizer_dir}/propose/result.json" 2>/dev/null || true)"
    if [[ "${propose_invalid}" == "true" ]]; then
      dorkpipe_orchestrate_operation_emit "orchestrate.optimize.iteration" "done" "$(dorkpipe_orchestrate_operation_duration_ms "${iteration_started_ms}")" \
        "iteration=${i}" \
        "iterations=${iterations}" \
        "target_workflow=${target_workflow}" \
        "child_workflow=${child_workflow}" \
        "status=invalid_patch" \
        "iter_dir=${iter_dir}"
      cat > "${result_json}" <<EOF
{
  "status": "stopped",
  "reason": "invalid_patch",
  "iterations": ${iterations},
  "completed_child_iterations": ${i},
  "run_dir": "${run_dir}"
}
EOF
      if [[ "${stop_on_invalid_patch}" =~ ^(1|true|yes|on)$ ]]; then
        exit 1
      fi
    else
      dorkpipe_orchestrate_operation_emit "orchestrate.optimize.iteration" "done" "$(dorkpipe_orchestrate_operation_duration_ms "${iteration_started_ms}")" \
        "iteration=${i}" \
        "iterations=${iterations}" \
        "target_workflow=${target_workflow}" \
        "child_workflow=${child_workflow}" \
        "status=ready" \
        "iter_dir=${iter_dir}"
    fi

    if [[ "${apply_status}" == "applied" && "${refresh_target_after_apply}" =~ ^(1|true|yes|on)$ ]]; then
      refresh_started_ms="$(dorkpipe_orchestrate_now_ms)"
      dorkpipe_orchestrate_operation_emit "orchestrate.optimize.refresh" "start" "" \
        "iteration=${i}" \
        "iterations=${iterations}" \
        "target_workflow=${target_workflow}"
      DORKPIPE_ORCH_APPROVAL_MODE=auto-no \
        DORKPIPE_ORCH_SKIP_APPLY=1 \
        DORKPIPE_DEV_STACK_RELOAD=1 \
        dockpipe "${target_args[@]}" --
      dorkpipe_orchestrate_operation_emit "orchestrate.optimize.refresh" "done" "$(dorkpipe_orchestrate_operation_duration_ms "${refresh_started_ms}")" \
        "iteration=${i}" \
        "iterations=${iterations}" \
        "target_workflow=${target_workflow}" \
        "status=done"
    fi
  done

  cat > "${result_json}" <<EOF
{
  "status": "ready",
  "iterations": ${iterations},
  "completed_child_iterations": $((iterations - 1)),
  "final_iteration": "current workflow",
  "child_package": "${child_package}",
  "child_workflow": "${child_workflow}",
  "run_dir": "${run_dir}"
}
EOF
  finish_optimizer_operation "done" "ready" \
    "iterations=${iterations}" \
    "completed_child_iterations=$((iterations - 1))" \
    "run_dir=${run_dir}"
  exit 0
fi

"$(dorkpipe_orchestrate_helper_bin)" optimize-action "${action}" "${ROOT}" "${target_dir}" "${optimizer_dir}" "${DORKPIPE_ORCH_ROOT}" "${DORKPIPE_ORCH_APPROVAL_MD}" "${result_json}"

result_status="$("$(dorkpipe_orchestrate_helper_bin)" optimizer-result-status "${result_json}" 2>/dev/null || true)"
finish_optimizer_operation "done" "${result_status:-ready}"
