#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
merge_json="${DORKPIPE_ORCH_MERGE_DIR}/result.json"
[[ -f "${merge_json}" ]] || { echo "missing merge result" >&2; exit 1; }
op_started_ms="$(dorkpipe_orchestrate_now_ms)"
dorkpipe_orchestrate_operation_emit "orchestrate.verify" "start" "" "workflow=${DORKPIPE_ORCH_WORKFLOW}" "root=${DORKPIPE_ORCH_VERIFY_DIR}"

status="pass"
confidence="0.60"
issues='[]'
next_action="human approval before treating orchestration output as final"

eval "$("$(dorkpipe_orchestrate_helper_bin)" verify-plan-env "${DORKPIPE_ORCH_PLAN_JSON}")"
next_action="${VERIFY_NEXT_ACTION_DEFAULT:-$next_action}"

eval "$("$(dorkpipe_orchestrate_helper_bin)" verify-summary-env "${merge_json}")"
confidence="${VERIFY_AVG_CONFIDENCE:-${confidence}}"
if [[ "${VERIFY_LIVE_COUNT:-0}" == "0" ]]; then
  status="review"
  issues='["all worker tasks used fallback output"]'
  next_action="fix live worker backends or enable governed escalation before applying output"
fi

eval "$("$(dorkpipe_orchestrate_helper_bin)" verify-heuristics "${merge_json}" "${DORKPIPE_ORCH_TASKS_DIR}" "${issues}")"
if [[ "${VERIFY_HEURISTIC_STATUS:-pass}" != "pass" ]]; then
  status="${VERIFY_HEURISTIC_STATUS}"
  issues="${VERIFY_HEURISTIC_ISSUES:-${issues}}"
  next_action="human review: worker output appears to miss the requested artifact shape"
fi

eval "$("$(dorkpipe_orchestrate_helper_bin)" verify-apply-coherence "${ROOT}" "${DORKPIPE_ORCH_ROOT}" "${DORKPIPE_ORCH_PLAN_JSON}" "${issues}")"
if [[ "${VERIFY_APPLY_STATUS:-pass}" != "pass" ]]; then
  status="${VERIFY_APPLY_STATUS}"
  issues="${VERIFY_APPLY_ISSUES:-${issues}}"
  next_action="repair staged apply outputs: broken links, bad references, and contradictory validation claims block apply"
fi

if [[ -f "${DORKPIPE_ORCH_HALT_JSON}" && "${status}" != "fail" ]]; then
  status="review"
  issues='["cloud budget halt triggered during orchestration; review cloud-usage.json and halt.json before resuming cloud workers"]'
  next_action="human review of cloud budget halt before any further cloud worker execution"
fi

if ! "$(dorkpipe_orchestrate_helper_bin)" build-verify-result \
  "${DORKPIPE_ORCH_VERIFY_DIR}/result.json" \
  "${DORKPIPE_ORCH_PLAN_JSON}" \
  "${DORKPIPE_ORCH_GRAPH_JSON}" \
  "${merge_json}" \
  "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" \
  "${DORKPIPE_ORCH_HALT_JSON}" \
  "${status}" \
  "${confidence}" \
  "${issues}" \
  "${next_action}"; then
  dorkpipe_orchestrate_operation_fail "orchestrate.verify" "${op_started_ms}" "build-verify-result failed" "workflow=${DORKPIPE_ORCH_WORKFLOW}"
  exit 1
fi

dorkpipe_orchestrate_operation_emit "orchestrate.verify" "done" "$(dorkpipe_orchestrate_operation_duration_ms "${op_started_ms}")" "workflow=${DORKPIPE_ORCH_WORKFLOW}" "result=${DORKPIPE_ORCH_VERIFY_DIR}/result.json" "status=${status}"
