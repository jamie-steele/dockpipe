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

python3 - "${DORKPIPE_ORCH_GRAPH_JSON}" "${SCRIPT_DIR}/orchestrate-run-task.sh" <<'PY'
import json
import os
import subprocess
import sys
import time

graph_path = sys.argv[1]
runner = sys.argv[2]
graph = json.load(open(graph_path, "r", encoding="utf-8"))
concurrency = graph.get("concurrency", {}) or {}
max_workers = max(1, int(concurrency.get("max_workers") or 1))
max_local_workers = max(1, int(concurrency.get("max_local_workers") or max_workers))
max_cloud_workers = max(1, int(concurrency.get("max_cloud_workers") or 1))

tasks = {}
for raw in graph.get("tasks", []):
    worker_type = str(raw.get("worker_type", "analysis"))
    task_id = str(raw.get("id", ""))
    if task_id and worker_type not in {"merge", "verify"}:
        tasks[task_id] = {
            "id": task_id,
            "depends_on": [str(dep) for dep in raw.get("depends_on", []) or []],
            "provider": str(raw.get("provider", "")),
        }

if not tasks:
    raise SystemExit(f"no runnable worker tasks in {graph_path}")

cloud_providers = {"codex", "claude"}
running = {}
done = set()
failed = {}
started = set()

def is_cloud(task):
    return task.get("provider") in cloud_providers

def active_counts():
    total = len(running)
    cloud = sum(1 for item in running.values() if is_cloud(item["task"]))
    local = total - cloud
    return total, local, cloud

def runnable():
    out = []
    total, local, cloud = active_counts()
    for task_id, task in tasks.items():
        if task_id in done or task_id in failed or task_id in started:
            continue
        if any(dep in failed for dep in task["depends_on"]):
            failed[task_id] = "dependency failed"
            continue
        if not all(dep in done for dep in task["depends_on"] if dep in tasks):
            continue
        if total >= max_workers:
            break
        if is_cloud(task):
            if cloud >= max_cloud_workers:
                continue
            cloud += 1
        else:
            if local >= max_local_workers:
                continue
            local += 1
        total += 1
        out.append(task)
    return out

while len(done) + len(failed) < len(tasks):
    launched = False
    for task in runnable():
        task_id = task["id"]
        proc = subprocess.Popen(["bash", runner, task_id], env=os.environ.copy())
        running[task_id] = {"process": proc, "task": task}
        started.add(task_id)
        launched = True
        print(f"[dorkpipe] started orchestration task {task_id} ({task.get('provider') or 'unknown'})", file=sys.stderr)

    if not running:
        if not launched:
            blocked = sorted(set(tasks) - done - set(failed))
            raise SystemExit(f"orchestration scheduler stalled; blocked tasks: {', '.join(blocked)}")

    finished = []
    for task_id, item in running.items():
        code = item["process"].poll()
        if code is not None:
            finished.append((task_id, code))

    if not finished:
        time.sleep(0.2)
        continue

    for task_id, code in finished:
        running.pop(task_id, None)
        if code == 0:
            done.add(task_id)
            print(f"[dorkpipe] completed orchestration task {task_id}", file=sys.stderr)
        else:
            failed[task_id] = f"exit status {code}"
            print(f"[dorkpipe] failed orchestration task {task_id}: exit status {code}", file=sys.stderr)

if failed:
    detail = ", ".join(f"{task_id} ({reason})" for task_id, reason in sorted(failed.items()))
    raise SystemExit(f"orchestration task failure(s): {detail}")

print(f"[dorkpipe] ran {len(done)} orchestration task(s) with max_workers={max_workers} max_local_workers={max_local_workers} max_cloud_workers={max_cloud_workers}", file=sys.stderr)
PY
