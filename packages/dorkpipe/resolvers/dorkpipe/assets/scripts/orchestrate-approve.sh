#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
op_started_ms="$(dorkpipe_orchestrate_now_ms)"
dorkpipe_orchestrate_operation_emit "orchestrate.approval" "start" "" "workflow=${DORKPIPE_ORCH_WORKFLOW}" "mode=${DORKPIPE_ORCH_APPROVAL_MODE:-prompt}"

dorkpipe_orchestrate_approval_main() {
  local decision approved
  decision="review"
  if [[ "${DORKPIPE_ORCH_APPROVAL_MODE:-prompt}" == "auto-no" ]]; then
    decision="review"
  elif [[ "${DORKPIPE_ORCH_APPROVAL_MODE:-prompt}" == "auto-yes" ]]; then
    decision="approve"
  else
    decision="$(dockpipe_sdk prompt choice \
    --id dorkpipe_orchestrate_approve \
    --title "Approve orchestration result?" \
    --message "Review ${DORKPIPE_ORCH_MERGE_DIR}/final.md and ${DORKPIPE_ORCH_VERIFY_DIR}/result.json. Choose whether this orchestration result is ready for manual follow-up." \
    --option review \
    --option approve \
    --default review \
    --intent review \
    --automation-group docs-review)" || return 1
  fi

  approved="no"
  if [[ "${decision}" == "approve" ]]; then
    approved="yes"
  fi

  cat > "${DORKPIPE_ORCH_APPROVAL_MD}" <<EOF
# Approval

- Decision: ${decision}
- Approved: ${approved}
- Final synthesis: \`${DORKPIPE_ORCH_MERGE_DIR}/final.md\`
- Verify result: \`${DORKPIPE_ORCH_VERIFY_DIR}/result.json\`

This step records human disposition only.
EOF
  dorkpipe_orchestrate_operation_emit "orchestrate.approval" "done" "$(dorkpipe_orchestrate_operation_duration_ms "${op_started_ms}")" "workflow=${DORKPIPE_ORCH_WORKFLOW}" "mode=${DORKPIPE_ORCH_APPROVAL_MODE:-prompt}" "decision=${decision}" "approved=${approved}" "approval=${DORKPIPE_ORCH_APPROVAL_MD}"
}

if ! dorkpipe_orchestrate_approval_main; then
  dorkpipe_orchestrate_operation_fail "orchestrate.approval" "${op_started_ms}" "approval recording failed" "workflow=${DORKPIPE_ORCH_WORKFLOW}" "mode=${DORKPIPE_ORCH_APPROVAL_MODE:-prompt}" "approval=${DORKPIPE_ORCH_APPROVAL_MD}"
  exit 1
fi
