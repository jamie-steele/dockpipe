#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:?DOCKPIPE_SCRIPT_DIR is required}"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
merge_json="${DORKPIPE_ORCH_MERGE_DIR}/result.json"
[[ -f "${merge_json}" ]] || { echo "missing merge result" >&2; exit 1; }

status="pass"
confidence="0.60"
issues='[]'
next_action="human approval before treating orchestration output as final"

eval "$(
  python3 - "${DORKPIPE_ORCH_PLAN_JSON}" <<'PY'
import json
import shlex
import sys

plan = json.load(open(sys.argv[1], "r", encoding="utf-8"))
verify = plan.get("verify", {})
print(f"VERIFY_NEXT_ACTION_DEFAULT={shlex.quote(verify.get('next_action_default', 'human approval before treating orchestration output as final'))}")
PY
)"
next_action="${VERIFY_NEXT_ACTION_DEFAULT:-$next_action}"

if command -v jq >/dev/null 2>&1; then
  live_count="$(jq '[.tasks[] | select(.used_live_model == true)] | length' "${merge_json}" 2>/dev/null || echo 0)"
  avg_conf="$(jq -r '.average_confidence // 0.6' "${merge_json}" 2>/dev/null || echo 0.6)"
  confidence="${avg_conf}"
  if [[ "${live_count}" == "0" ]]; then
    issues='["all worker tasks used fallback output"]'
  fi
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
