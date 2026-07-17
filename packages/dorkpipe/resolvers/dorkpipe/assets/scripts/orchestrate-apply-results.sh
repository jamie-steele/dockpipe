#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
[[ -f "${DORKPIPE_ORCH_PLAN_JSON}" ]] || { echo "missing plan artifact: ${DORKPIPE_ORCH_PLAN_JSON}" >&2; exit 1; }
op_started_ms="$(dorkpipe_orchestrate_now_ms)"
dorkpipe_orchestrate_operation_emit "orchestrate.apply" "start" "" "workflow=${DORKPIPE_ORCH_WORKFLOW}" "root=${DORKPIPE_ORCH_APPLY_DIR}"

if [[ "${DORKPIPE_ORCH_SKIP_APPLY:-0}" =~ ^(1|true|yes|on)$ ]]; then
  mkdir -p "${DORKPIPE_ORCH_APPLY_DIR}"
  cat > "${DORKPIPE_ORCH_APPLY_DIR}/result.json" <<EOF
{
  "status": "skipped",
  "reason": "DORKPIPE_ORCH_SKIP_APPLY is enabled",
  "applied": []
}
EOF
  dorkpipe_orchestrate_operation_emit "orchestrate.apply" "done" "$(dorkpipe_orchestrate_operation_duration_ms "${op_started_ms}")" "status=skipped" "result=${DORKPIPE_ORCH_APPLY_DIR}/result.json"
  exit 0
fi

if ! "$(dorkpipe_orchestrate_helper_bin)" apply-results "${ROOT}" "${DORKPIPE_ORCH_ROOT}" "${DORKPIPE_ORCH_PLAN_JSON}" "${DORKPIPE_ORCH_APPROVAL_MD}" "${DORKPIPE_ORCH_APPLY_DIR}/result.json" "${DORKPIPE_ORCH_VERIFY_DIR}/result.json"; then
  dorkpipe_orchestrate_operation_fail "orchestrate.apply" "${op_started_ms}" "apply-results failed" "workflow=${DORKPIPE_ORCH_WORKFLOW}" "root=${DORKPIPE_ORCH_APPLY_DIR}"
  exit 1
fi

dorkpipe_orchestrate_operation_emit "orchestrate.apply" "done" "$(dorkpipe_orchestrate_operation_duration_ms "${op_started_ms}")" "workflow=${DORKPIPE_ORCH_WORKFLOW}" "result=${DORKPIPE_ORCH_APPLY_DIR}/result.json"
