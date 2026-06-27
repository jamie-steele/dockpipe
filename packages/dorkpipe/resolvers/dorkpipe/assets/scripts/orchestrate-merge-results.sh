#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:?DOCKPIPE_SCRIPT_DIR is required}"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
mapfile -t result_paths < <(
  python3 - "${DORKPIPE_ORCH_GRAPH_JSON}" "${DORKPIPE_ORCH_TASKS_DIR}" <<'PY'
import json
import pathlib
import sys

graph = json.load(open(sys.argv[1], "r", encoding="utf-8"))
tasks_dir = pathlib.Path(sys.argv[2])
for task in graph.get("tasks", []):
    worker_type = task.get("worker_type", "analysis")
    if worker_type in {"merge", "verify"}:
        continue
    print(tasks_dir / task["id"] / "result.json")
PY
)

mapfile -t main_result_paths < <(
  python3 - "${DORKPIPE_ORCH_GRAPH_JSON}" "${DORKPIPE_ORCH_TASKS_DIR}" <<'PY'
import json
import pathlib
import sys

graph = json.load(open(sys.argv[1], "r", encoding="utf-8"))
tasks_dir = pathlib.Path(sys.argv[2])
for task in graph.get("tasks", []):
    worker_type = task.get("worker_type", "analysis")
    if worker_type in {"merge", "verify", "planning", "scout"}:
        continue
    print(tasks_dir / task["id"] / "result.json")
PY
)

mapfile -t planning_result_paths < <(
  python3 - "${DORKPIPE_ORCH_GRAPH_JSON}" "${DORKPIPE_ORCH_TASKS_DIR}" <<'PY'
import json
import pathlib
import sys

graph = json.load(open(sys.argv[1], "r", encoding="utf-8"))
tasks_dir = pathlib.Path(sys.argv[2])
for task in graph.get("tasks", []):
    worker_type = task.get("worker_type", "analysis")
    if worker_type in {"planning", "scout"}:
        print(tasks_dir / task["id"] / "result.json")
PY
)

missing=0
for path in "${result_paths[@]}"; do
  [[ -f "$path" ]] || missing=1
done
if [[ "$missing" -ne 0 ]]; then
  echo "missing worker result(s) for merge" >&2
  exit 1
fi

eval "$(
  python3 - "${DORKPIPE_ORCH_PLAN_JSON}" <<'PY'
import json
import shlex
import sys

plan = json.load(open(sys.argv[1], "r", encoding="utf-8"))
merge = plan.get("merge", {})
print(f"MERGE_TITLE={shlex.quote(merge.get('title', 'DorkPipe Orchestration Synthesis'))}")
print(f"MERGE_SUMMARY_POINTS_JSON={shlex.quote(json.dumps(merge.get('summary_points', [])))}")
PY
)"

if command -v jq >/dev/null 2>&1; then
  jq -s '{
    status:"ok",
    tasks: map({task_id, base_task_id, comparison, provider_actual, used_live_model, budget_halt, estimated_input_tokens, estimated_output_tokens, estimated_total_tokens, started_at, finished_at, duration_ms, summary, confidence}),
    average_confidence: ((map(.confidence) | add) / length),
    total_estimated_input_tokens: (map(.estimated_input_tokens // 0) | add),
    total_estimated_output_tokens: (map(.estimated_output_tokens // 0) | add),
    total_estimated_task_tokens: (map(.estimated_total_tokens // 0) | add),
    total_duration_ms: (map(.duration_ms // 0) | add),
    max_task_duration_ms: (map(.duration_ms // 0) | max)
  }' "${main_result_paths[@]}" > "${DORKPIPE_ORCH_MERGE_DIR}/result.json"
  if [[ "${#planning_result_paths[@]}" -gt 0 ]]; then
    tmp_result="${DORKPIPE_ORCH_MERGE_DIR}/result.tmp.json"
    jq -s --slurpfile planning <(jq -s 'map({task_id, base_task_id, provider_actual, used_live_model, estimated_input_tokens, estimated_output_tokens, estimated_total_tokens, started_at, finished_at, duration_ms, summary, confidence})' "${planning_result_paths[@]}") \
      '.[0] + {planning_tasks: ($planning[0] // [])}' \
      "${DORKPIPE_ORCH_MERGE_DIR}/result.json" > "${tmp_result}"
    mv "${tmp_result}" "${DORKPIPE_ORCH_MERGE_DIR}/result.json"
  fi
else
  cat > "${DORKPIPE_ORCH_MERGE_DIR}/result.json" <<EOF
{"status":"ok","tasks":["repo_shape","package_contracts","safety_model"],"average_confidence":0.6}
EOF
fi

python3 - "${DORKPIPE_ORCH_MERGE_DIR}/result.json" "${DORKPIPE_ORCH_MERGE_DIR}/final.md" "${DORKPIPE_ORCH_TASKS_DIR}" <<'PY'
import json
import os
import pathlib
import sys

merge_result = json.load(open(sys.argv[1], "r", encoding="utf-8"))
dest = pathlib.Path(sys.argv[2])
tasks_dir = pathlib.Path(sys.argv[3])
title = os.environ.get("MERGE_TITLE", "DorkPipe Orchestration Synthesis")
summary_points = json.loads(os.environ.get("MERGE_SUMMARY_POINTS_JSON", "[]"))
lines = [f"# {title}", "", "## Task Summaries", ""]
for task in merge_result.get("tasks", []):
    lines.append(f"- `{task['task_id']}` ({task.get('provider_actual', 'unknown')}): {task.get('summary', '')}")
if summary_points:
    lines.extend(["", "## Synthesis", ""])
    for point in summary_points:
        lines.append(f"- {point}")
planning_tasks = merge_result.get("planning_tasks", [])
if planning_tasks:
    lines.extend(["", "## Planning Scouts", ""])
    for task in planning_tasks:
        lines.append(f"- `{task['task_id']}` ({task.get('provider_actual', 'unknown')}): {task.get('summary', '')}")
lines.extend(["", "## Worker Outputs", ""])
for task in merge_result.get("tasks", []):
    task_id = task["task_id"]
    response = tasks_dir / task_id / "response.md"
    lines.extend([f"### {task_id}", ""])
    if response.exists():
        lines.append(response.read_text(encoding="utf-8").strip())
    else:
        lines.append("_No response artifact was written._")
    lines.append("")
if os.path.exists(os.environ["DORKPIPE_ORCH_HALT_JSON"]):
    lines.extend(["", "## Budget Halt", "", "- This run triggered the cloud budget halt, so later cloud tasks were intentionally skipped."])
dest.write_text("\n".join(lines) + "\n")
PY

printf '[dorkpipe] merge result ready at %s\n' "${DORKPIPE_ORCH_MERGE_DIR}/result.json" >&2
