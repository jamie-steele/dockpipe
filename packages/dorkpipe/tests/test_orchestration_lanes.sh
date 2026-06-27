#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
export PATH="$ROOT/src/bin:$PATH"
export DOCKPIPE_SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
export DOCKPIPE_ASSETS_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets"
export DOCKPIPE_WORKFLOW_CONFIG="$ROOT/packages/agent/workflows/docs.orchestrate/config.yml"
export DOCKPIPE_WORKFLOW_NAME="docs.orchestrate"
export DOCKPIPE_STEP_ID="plan"
export DORKPIPE_ORCH_WORKFLOW="test.docs.orchestrate"
export DORKPIPE_ORCH_ROOT="${TMPDIR:-/tmp}/dorkpipe-orch-lanes-${RANDOM}-${RANDOM}"
export DORKPIPE_ORCH_LIVE_MODELS="false"
export DORKPIPE_ORCH_TRAINING_MODE="observe"
export DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS="1000"
export DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS="400"

bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-plan.sh" >/dev/null
bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-run-tasks.sh" >/dev/null

python3 - "$DORKPIPE_ORCH_ROOT" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
lane_plan = json.loads((root / "lanes" / "plan.json").read_text())
tasks = lane_plan.get("tasks", [])
assert len(tasks) == 3, f"expected 3 planned tasks, got {len(tasks)}"
providers = {task["task_id"]: task["provider"] for task in tasks}
assert providers["repo_shape"] == "ollama", providers
assert providers["package_contracts"] == "ollama", providers
assert providers["safety_model"] == "ollama", providers
assert all(task.get("gated_by_baseline") or task["task_id"] == "repo_shape" for task in tasks), tasks
metrics = (root / "training" / "metrics.jsonl").read_text().strip().splitlines()
assert len(metrics) == 3, f"expected 3 training metrics, got {len(metrics)}"
for line in metrics:
    metric = json.loads(line)
    assert metric["used_live_model"] is False
    assert metric["training_mode"] == "observe"
for task_id in providers:
    result = json.loads((root / "tasks" / task_id / "result.json").read_text())
    assert result["lane_id"], result
    assert result["lane_selection"]["task_id"] == task_id, result
PY

echo "test_orchestration_lanes OK"
