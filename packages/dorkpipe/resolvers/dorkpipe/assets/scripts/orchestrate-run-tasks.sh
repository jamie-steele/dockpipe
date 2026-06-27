#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:?DOCKPIPE_SCRIPT_DIR is required}"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
[[ -f "${DORKPIPE_ORCH_GRAPH_JSON}" ]] || {
  echo "missing task graph: ${DORKPIPE_ORCH_GRAPH_JSON}" >&2
  exit 1
}

mapfile -t task_ids < <(
  python3 - "${DORKPIPE_ORCH_GRAPH_JSON}" <<'PY'
import json
import sys

graph = json.load(open(sys.argv[1], "r", encoding="utf-8"))
for task in graph.get("tasks", []):
    worker_type = str(task.get("worker_type", ""))
    task_id = str(task.get("id", ""))
    if task_id and worker_type not in {"merge", "verify"}:
        print(task_id)
PY
)

if [[ ${#task_ids[@]} -eq 0 ]]; then
  echo "no runnable worker tasks in ${DORKPIPE_ORCH_GRAPH_JSON}" >&2
  exit 1
fi

for task_id in "${task_ids[@]}"; do
  bash "${SCRIPT_DIR}/orchestrate-run-task.sh" "${task_id}"
done

printf '[dorkpipe] ran %s orchestration task(s)\n' "${#task_ids[@]}" >&2
