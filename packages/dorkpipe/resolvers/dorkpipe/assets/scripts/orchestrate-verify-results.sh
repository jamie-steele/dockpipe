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

eval "$(
  python3 - "${merge_json}" "${DORKPIPE_ORCH_TASKS_DIR}" "${issues}" <<'PY'
import json
import pathlib
import re
import shlex
import sys

merge = json.load(open(sys.argv[1], "r", encoding="utf-8"))
tasks_dir = pathlib.Path(sys.argv[2])
try:
    issues = json.loads(sys.argv[3])
except Exception:
    issues = []

bad_patterns = [
    (re.compile(r"\bI will (outline|create|write|complete)\b", re.I), "worker narrated a plan instead of returning the requested artifact"),
    (re.compile(r"(?im)^\s*(#{1,6}\s*)?(Task Artifact|Lane Selection|Worker Result Artifact|Merge Result|Final Report Checklist)\s*:"), "worker imitated orchestration artifacts instead of answering the task"),
    (re.compile(r"```json\s*\{", re.I), "worker returned sample JSON artifacts instead of concise markdown"),
    (re.compile(r"\b(files? (were|modified|touched)|validations? run|generated artifacts?)\b", re.I), "worker included implementation/reporting chatter"),
]

boundary_patterns = [
    (re.compile(r"\bworkflow declares (?:its )?limitations in concurrency control\b", re.I), "worker incorrectly said workflow does not own concurrency declaration"),
    (re.compile(r"\bworkflow (?:does not|should not|is not responsible to) own concurrency\b", re.I), "worker incorrectly said workflow does not own concurrency declaration"),
    (re.compile(r"\bconcurrency (?:is|should be) (?:owned|managed) by worker results\b", re.I), "worker incorrectly assigned concurrency declaration to worker results"),
]

shape_patterns = [
    (re.compile(r"(?im)^\s*Here (?:are|is)\b"), "worker included preamble instead of direct artifact content"),
    (re.compile(r"(?im)^###\s+repo_shape\s*$"), "worker repeated task id as a heading"),
    (re.compile(r"\buncertainties remain\b", re.I), "worker added generic uncertainty instead of bounded uncertainty"),
    (re.compile(r"\b(?:lane scores|confidence values) should be cited\b", re.I), "worker invented lane score citation guidance"),
]

for task in list(merge.get("planning_tasks", [])) + list(merge.get("tasks", [])):
    task_id = task.get("task_id")
    if not task_id:
        continue
    response_path = tasks_dir / task_id / "response.md"
    if not response_path.exists():
        issues.append(f"{task_id}: response artifact is missing")
        continue
    text = response_path.read_text(encoding="utf-8", errors="replace")
    for pattern, message in bad_patterns:
        if pattern.search(text):
            issues.append(f"{task_id}: {message}")
            break
    for pattern, message in boundary_patterns:
        if pattern.search(text):
            issues.append(f"{task_id}: {message}")
            break
    for pattern, message in shape_patterns:
        if pattern.search(text):
            issues.append(f"{task_id}: {message}")
            break

status = "pass"
if issues:
    status = "review"
print(f"VERIFY_HEURISTIC_STATUS={shlex.quote(status)}")
print(f"VERIFY_HEURISTIC_ISSUES={shlex.quote(json.dumps(issues))}")
PY
)"
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
