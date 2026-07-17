#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
[[ -d "${DORKPIPE_ORCH_TASKS_DIR}" ]] || {
  echo "missing task directory: ${DORKPIPE_ORCH_TASKS_DIR}" >&2
  exit 1
}
op_started_ms="$(dorkpipe_orchestrate_now_ms)"
dorkpipe_orchestrate_operation_emit "orchestrate.auth.preflight" "start" "" "workflow=${DORKPIPE_ORCH_WORKFLOW}"

mapfile -t required_providers < <("$(dorkpipe_orchestrate_helper_bin)" required-auth-providers "${DORKPIPE_ORCH_TASKS_DIR}")
if ((${#required_providers[@]} == 0)); then
  dorkpipe_orchestrate_operation_emit "orchestrate.auth.preflight" "done" "$(dorkpipe_orchestrate_operation_duration_ms "${op_started_ms}")" "workflow=${DORKPIPE_ORCH_WORKFLOW}" "providers=none"
  exit 0
fi

for provider in "${required_providers[@]}"; do
  [[ -n "${provider}" ]] || continue
  if ! dorkpipe_orchestrate_auth_preflight "${provider}"; then
    dorkpipe_orchestrate_operation_fail "orchestrate.auth.preflight" "${op_started_ms}" "provider auth preflight failed" "workflow=${DORKPIPE_ORCH_WORKFLOW}" "provider=${provider}"
    exit 1
  fi
done

dorkpipe_orchestrate_operation_emit "orchestrate.auth.preflight" "done" "$(dorkpipe_orchestrate_operation_duration_ms "${op_started_ms}")" "workflow=${DORKPIPE_ORCH_WORKFLOW}" "providers=${#required_providers[@]}"
