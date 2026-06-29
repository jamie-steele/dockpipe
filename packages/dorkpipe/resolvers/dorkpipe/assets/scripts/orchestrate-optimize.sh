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
  if [[ "${path}" = /* ]]; then
    printf '%s\n' "${path}"
  else
    printf '%s/%s\n' "${ROOT}" "${path}"
  fi
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
    printf '[dorkpipe] optimizer %s result ready at %s\n' "${action}" "${result_json}" >&2
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
    printf '\n[dorkpipe] optimizer iteration %02d/%02d\n' "${i}" "${iterations}" >&2
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
      printf '[dorkpipe] optimizer iteration %02d stopped: Codex proposed an invalid patch\n' "${i}" >&2
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
    fi

    if [[ "${apply_status}" == "applied" && "${refresh_target_after_apply}" =~ ^(1|true|yes|on)$ ]]; then
      printf '[dorkpipe] optimizer iteration %02d/%02d refreshing target workflow %s\n' "${i}" "${iterations}" "${target_workflow}" >&2
      DORKPIPE_ORCH_APPROVAL_MODE=auto-no \
        DORKPIPE_ORCH_SKIP_APPLY=1 \
        DORKPIPE_DEV_STACK_RELOAD=1 \
        dockpipe "${target_args[@]}" --
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
  printf '[dorkpipe] optimizer %s result ready at %s\n' "${action}" "${result_json}" >&2
  exit 0
fi

"$(dorkpipe_orchestrate_helper_bin)" optimize-action "${action}" "${ROOT}" "${target_dir}" "${optimizer_dir}" "${DORKPIPE_ORCH_ROOT}" "${DORKPIPE_ORCH_APPROVAL_MD}" "${result_json}"

printf '[dorkpipe] optimizer %s result ready at %s\n' "${action}" "${result_json}" >&2
