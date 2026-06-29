#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
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
        comparison = raw.get("comparison") or {}
        tasks[task_id] = {
            "id": task_id,
            "base_task_id": str(raw.get("base_task_id") or task_id),
            "comparison": comparison,
            "depends_on": [str(dep) for dep in raw.get("depends_on", []) or []],
            "provider": str(raw.get("provider", "")),
            "model": str(raw.get("model", "")),
        }

if not tasks:
    raise SystemExit(f"no runnable worker tasks in {graph_path}")

cloud_providers = {"codex", "claude"}
running = {}
done = set()
failed = {}
started = set()
started_at = {}
finished_at = {}
worker_logs = {}
task_results = {}
last_render = 0.0
frame_index = 0
rendered_lines = 0

animation_pref = os.environ.get("DORKPIPE_ORCH_COMPARE_ANIMATION", "auto").lower()
renderer = os.environ.get("DORKPIPE_ORCH_COMPARE_RENDERER", "clear").lower()
worker_log_mode = os.environ.get("DORKPIPE_ORCH_COMPARE_WORKER_LOGS", "artifact").lower()
render_interval = float(os.environ.get("DORKPIPE_ORCH_COMPARE_ANIMATION_INTERVAL", "0.35") or "0.35")
has_comparison = any((task.get("comparison") or {}).get("enabled") for task in tasks.values())
animation_enabled = has_comparison and animation_pref != "false" and animation_pref != "0"
if animation_pref == "auto":
    animation_enabled = animation_enabled and sys.stderr.isatty()
if renderer not in {"clear", "inline"}:
    renderer = "clear"
if worker_log_mode not in {"artifact", "terminal"}:
    worker_log_mode = "artifact"

def is_cloud(task):
    return task.get("provider") in cloud_providers

def task_status(task_id):
    if task_id in failed:
        return "failed"
    if task_id in done:
        return "done"
    if task_id in running:
        return "running"
    if task_id in started:
        return "started"
    return "queued"

def read_task_result(task_id):
    if task_id in task_results:
        return task_results[task_id]
    root = os.environ.get("DORKPIPE_ORCH_TASKS_DIR", "")
    if not root:
        return {}
    path = os.path.join(root, task_id, "result.json")
    try:
        with open(path, "r", encoding="utf-8") as handle:
            task_results[task_id] = json.load(handle)
    except Exception:
        task_results[task_id] = {}
    return task_results[task_id]

def format_elapsed(task_id, now):
    start = started_at.get(task_id)
    if not start:
        return "  --.-s"
    end = finished_at.get(task_id, now)
    return f"{end - start:6.1f}s"

def format_tokens(task_id):
    result = read_task_result(task_id)
    tokens = int(result.get("estimated_total_tokens") or 0)
    if tokens <= 0:
        return "    -- tok"
    if tokens >= 1000:
        return f"{tokens / 1000:6.1f}k tok"
    return f"{tokens:6d} tok"

def fighter_label(task_id):
    task = tasks[task_id]
    provider = (task.get("provider") or "unknown").upper()
    model = task.get("model") or ""
    if model:
        return f"{provider}({model})"
    return provider

def fighter_bar(task_id, now, frame):
    status = task_status(task_id)
    if status == "done":
        return "[##########]"
    if status == "failed":
        return "[xxx   xxx]"
    if status == "queued":
        return "[          ]"
    patterns = [
        "[>        <]",
        "[=>      <=]",
        "[==>    <==]",
        "[===>  <===]",
        "[==>    <==]",
        "[=>      <=]",
    ]
    return patterns[frame % len(patterns)]

def comparison_groups():
    groups = {}
    for task_id, task in tasks.items():
        comparison = task.get("comparison") or {}
        if not comparison.get("enabled"):
            continue
        base = task.get("base_task_id") or comparison.get("base_task_id") or task_id
        groups.setdefault(base, []).append(task_id)
    return {base: ids for base, ids in groups.items() if len(ids) >= 2}

def comparison_task_ids():
    ids = set()
    for group in comparison_groups().values():
        ids.update(group)
    return ids

def local_summary():
    comparison_ids = comparison_task_ids()
    local_ids = [task_id for task_id in tasks if task_id not in comparison_ids]
    if not local_ids:
        return ""
    parts = []
    for task_id in sorted(local_ids):
        task = tasks[task_id]
        parts.append(f"{task_id}:{fighter_label(task_id)}:{task_status(task_id)}")
    return "local scout " + "  ".join(parts)

def worker_log_path(task_id):
    root = os.environ.get("DORKPIPE_ORCH_TASKS_DIR", "")
    if not root:
        return ""
    return os.path.join(root, task_id, "worker.log")

def repaint(lines):
    global rendered_lines
    if renderer == "clear":
        sys.stderr.write("\033[?25l\033[2J\033[H")
    elif rendered_lines:
        sys.stderr.write(f"\033[{rendered_lines}F")
        for _ in range(rendered_lines):
            sys.stderr.write("\033[2K\033[1E")
        sys.stderr.write(f"\033[{rendered_lines}F")
    else:
        sys.stderr.write("\033[?25l")
    sys.stderr.write("\n".join(lines) + "\n")
    rendered_lines = len(lines)
    sys.stderr.flush()

def render_fight(force=False):
    global last_render, frame_index
    if not animation_enabled:
        return
    now = time.time()
    if not force and now - last_render < render_interval:
        return
    last_render = now
    frame_index += 1
    lines = [
        "DorkPipe comparison lanes",
        "=========================",
        "",
    ]
    for base, ids in sorted(comparison_groups().items()):
        lines.append(f"{base}")
        for task_id in sorted(ids, key=lambda item: tasks[item].get("provider", item)):
            status = task_status(task_id)
            lines.append(
                f"  {fighter_label(task_id):<18} {fighter_bar(task_id, now, frame_index)} "
                f"{status:<7} {format_elapsed(task_id, now)} {format_tokens(task_id)}"
            )
        lines.append("            VS")
        lines.append("")
    scout = local_summary()
    if scout:
        lines.append(scout)
    comparison_ids = comparison_task_ids()
    comparison_done = len([task_id for task_id in comparison_ids if task_id in done])
    lines.append(
        f"comparison {comparison_done}/{len(comparison_ids)}  "
        f"total {len(done)}/{len(tasks)}  failed {len(failed)}  running {len(running)}"
    )
    repaint(lines)

def close_fight():
    if animation_enabled:
        render_fight(force=True)
        sys.stderr.write("\033[?25h\n")
        sys.stderr.flush()

def print_failure_log(task_id):
    path = worker_log_path(task_id)
    if not path or not os.path.exists(path):
        return
    try:
        lines = open(path, "r", encoding="utf-8", errors="replace").read().splitlines()
    except Exception:
        return
    if not lines:
        return
    print(f"[dorkpipe] worker log tail for {task_id} ({path}):", file=sys.stderr)
    for line in lines[-40:]:
        print(line, file=sys.stderr)

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
        log_handle = None
        if animation_enabled and worker_log_mode == "artifact":
            log_path = worker_log_path(task_id)
            if log_path:
                os.makedirs(os.path.dirname(log_path), exist_ok=True)
                log_handle = open(log_path, "a", encoding="utf-8")
                log_handle.write(f"[dorkpipe] starting {task_id} ({task.get('provider') or 'unknown'})\n")
                log_handle.flush()
        proc = subprocess.Popen(
            ["bash", runner, task_id],
            env=os.environ.copy(),
            stdout=log_handle if log_handle else None,
            stderr=subprocess.STDOUT if log_handle else None,
        )
        running[task_id] = {"process": proc, "task": task, "log_handle": log_handle}
        started.add(task_id)
        started_at[task_id] = time.time()
        launched = True
        if not animation_enabled:
            print(f"[dorkpipe] started orchestration task {task_id} ({task.get('provider') or 'unknown'})", file=sys.stderr)
        render_fight(force=True)

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
        render_fight()
        time.sleep(0.2)
        continue

    for task_id, code in finished:
        item = running.pop(task_id, None) or {}
        log_handle = item.get("log_handle")
        if log_handle:
            log_handle.write(f"[dorkpipe] finished {task_id} with exit status {code}\n")
            log_handle.close()
        finished_at[task_id] = time.time()
        if code == 0:
            done.add(task_id)
            if not animation_enabled:
                print(f"[dorkpipe] completed orchestration task {task_id}", file=sys.stderr)
        else:
            failed[task_id] = f"exit status {code}"
            if not animation_enabled:
                print(f"[dorkpipe] failed orchestration task {task_id}: exit status {code}", file=sys.stderr)
        render_fight(force=True)

if failed:
    close_fight()
    if animation_enabled:
        for task_id in sorted(failed):
            print_failure_log(task_id)
    detail = ", ".join(f"{task_id} ({reason})" for task_id, reason in sorted(failed.items()))
    raise SystemExit(f"orchestration task failure(s): {detail}")

close_fight()
print(f"[dorkpipe] ran {len(done)} orchestration task(s) with max_workers={max_workers} max_local_workers={max_local_workers} max_cloud_workers={max_cloud_workers}", file=sys.stderr)
PY
