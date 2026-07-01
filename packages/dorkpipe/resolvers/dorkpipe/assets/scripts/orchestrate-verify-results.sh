#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
merge_json="${DORKPIPE_ORCH_MERGE_DIR}/result.json"
[[ -f "${merge_json}" ]] || { echo "missing merge result" >&2; exit 1; }

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

if [[ -f "${DORKPIPE_ORCH_HALT_JSON}" ]]; then
  status="review"
  issues='["cloud budget halt triggered during orchestration; review cloud-usage.json and halt.json before resuming cloud workers"]'
  next_action="human review of cloud budget halt before any further cloud worker execution"
fi

cat > "${DORKPIPE_ORCH_VERIFY_DIR}/result.json" <<EOF
{
  "status": "${status}",
  "confidence": ${confidence},
  "issues": ${issues},
  "cloud_usage_artifact": "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}",
  "halt_artifact": "${DORKPIPE_ORCH_HALT_JSON}",
  "next_action": "$(dorkpipe_orchestrate_json_escape "${next_action}")"
}
EOF

printf '[dorkpipe] verify result ready at %s\n' "${DORKPIPE_ORCH_VERIFY_DIR}/result.json" >&2
