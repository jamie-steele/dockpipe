#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
export PATH="$ROOT/src/bin:$PATH"
export DOCKPIPE_SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
export DOCKPIPE_ASSETS_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets"
export DOCKPIPE_WORKFLOW_CONFIG="$ROOT/workflows/agent/docs.orchestrate/config.yml"
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
assert len(tasks) == 6, f"expected 6 planned tasks, got {len(tasks)}"
providers = {task["task_id"]: task["provider"] for task in tasks}
assert providers["contract_brain"] == "ollama", providers
assert providers["workflow_brain"] == "ollama", providers
assert providers["planner_brain"] == "ollama", providers
assert providers["repo_shape"] == "ollama", providers
assert providers["package_contracts"] == "ollama", providers
assert providers["safety_model"] == "ollama", providers
explicit_local_tasks = {"contract_brain", "workflow_brain", "planner_brain", "repo_shape"}
assert all(task.get("gated_by_baseline") or task["task_id"] in explicit_local_tasks for task in tasks), tasks
metrics = (root / "training" / "metrics.jsonl").read_text().strip().splitlines()
assert len(metrics) == 6, f"expected 6 training metrics, got {len(metrics)}"
for line in metrics:
    metric = json.loads(line)
    assert metric["used_live_model"] is False
    assert metric["training_mode"] == "observe"
    assert "estimated_total_tokens" in metric, metric
    assert "started_at" in metric and metric["started_at"], metric
    assert "finished_at" in metric and metric["finished_at"], metric
    assert isinstance(metric.get("duration_ms"), int), metric
for task_id in providers:
    result = json.loads((root / "tasks" / task_id / "result.json").read_text())
    assert result["lane_id"], result
    assert result["lane_selection"]["task_id"] == task_id, result
    assert "started_at" in result and result["started_at"], result
    assert "finished_at" in result and result["finished_at"], result
    assert isinstance(result.get("duration_ms"), int), result
prompt = (root / "tasks" / "package_contracts" / "prompt.md").read_text()
assert "Dependency context from completed upstream tasks:" in prompt, prompt
assert "### planner_brain" in prompt, prompt
assert "### contract_brain" not in prompt, prompt
assert "### workflow_brain" not in prompt, prompt
assert "AGENTS.md context:" in prompt, prompt
assert "DockPipe Root Router" in prompt, prompt
graph = json.loads((root / "task-graph.json").read_text())
graph_tasks = {task["id"]: task for task in graph["tasks"]}
assert graph_tasks["contract_brain"]["worker_type"] == "planning", graph_tasks["contract_brain"]
assert graph_tasks["workflow_brain"]["worker_type"] == "planning", graph_tasks["workflow_brain"]
assert graph_tasks["planner_brain"]["worker_type"] == "planning", graph_tasks["planner_brain"]
assert graph_tasks["planner_brain"]["depends_on"] == ["contract_brain", "workflow_brain"], graph_tasks["planner_brain"]
assert "planner_brain" in graph_tasks["package_contracts"]["depends_on"], graph_tasks["package_contracts"]
PY

echo "test_orchestration_lanes OK"

export DORKPIPE_ORCH_WORKFLOW="test.docs.orchestrate.force-codex"
export DORKPIPE_ORCH_ROOT="${TMPDIR:-/tmp}/dorkpipe-orch-force-codex-${RANDOM}-${RANDOM}"
export DORKPIPE_ORCH_FORCE_PROVIDER="codex"
export DORKPIPE_ORCH_CLOUD_LANES="true"

bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-plan.sh" >/dev/null

python3 - "$DORKPIPE_ORCH_ROOT" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
lane_plan = json.loads((root / "lanes" / "plan.json").read_text())
tasks = {task["task_id"]: task for task in lane_plan.get("tasks", [])}
assert tasks["contract_brain"]["provider"] == "ollama", tasks
assert tasks["workflow_brain"]["provider"] == "ollama", tasks
assert tasks["planner_brain"]["provider"] == "ollama", tasks
assert tasks["repo_shape"]["provider"] == "ollama", tasks
assert tasks["repo_shape"]["requested"] == "ollama", tasks
for task_id in ("package_contracts", "safety_model"):
    task = tasks[task_id]
    assert task["requested"] == "codex", task
    assert task["provider"] == "codex", task
    assert task["lane_id"] == "codex.cli.default", task
request = json.loads((root / "request.json").read_text())
assert request["force_provider"] == "codex", request
assert request["force_provider_scope"] == "auto", request
PY

echo "test_orchestration_force_codex OK"

export DORKPIPE_ORCH_WORKFLOW="test.docs.orchestrate.brain-codex"
export DORKPIPE_ORCH_ROOT="${TMPDIR:-/tmp}/dorkpipe-orch-brain-codex-${RANDOM}-${RANDOM}"
export DORKPIPE_ORCH_FORCE_PROVIDER=""
export DORKPIPE_ORCH_BRAIN_PROVIDER="codex"
export DORKPIPE_ORCH_CLOUD_LANES="true"

bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-plan.sh" >/dev/null

python3 - "$DORKPIPE_ORCH_ROOT" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
graph = json.loads((root / "task-graph.json").read_text())
tasks = {task["id"]: task for task in graph["tasks"]}
assert tasks["contract_brain"]["provider"] == "ollama", tasks["contract_brain"]
assert tasks["workflow_brain"]["provider"] == "ollama", tasks["workflow_brain"]
assert tasks["planner_brain"]["provider"] == "codex", tasks["planner_brain"]
assert tasks["repo_shape"]["provider"] == "ollama", tasks["repo_shape"]
assert tasks["package_contracts"]["provider"] == "ollama", tasks["package_contracts"]
assert tasks["safety_model"]["provider"] == "ollama", tasks["safety_model"]
assert tasks["planner_brain"]["depends_on"] == ["contract_brain", "workflow_brain"], tasks["planner_brain"]
for task_id in ("repo_shape", "package_contracts", "safety_model"):
    assert "planner_brain" in tasks[task_id]["depends_on"], tasks[task_id]
PY

echo "test_orchestration_brain_provider_codex OK"

export DORKPIPE_ORCH_WORKFLOW="test.docs.orchestrate.compare"
export DORKPIPE_ORCH_ROOT="${TMPDIR:-/tmp}/dorkpipe-orch-compare-${RANDOM}-${RANDOM}"
export DORKPIPE_ORCH_FORCE_PROVIDER=""
export DORKPIPE_ORCH_BRAIN_PROVIDER=""
export DORKPIPE_ORCH_COMPARE_PROVIDERS="ollama,codex,claude"
export DORKPIPE_ORCH_COMPARE_SCOPE="auto"
export DORKPIPE_ORCH_CLOUD_LANES="true"
export DORKPIPE_ORCH_CODEX_MODEL="test-codex-model"
export DORKPIPE_ORCH_CLAUDE_MODEL="test-claude-model"
export DORKPIPE_ORCH_OLLAMA_MODEL="test-ollama-model"

bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-plan.sh" >/dev/null

python3 - "$DORKPIPE_ORCH_ROOT" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
lane_plan = json.loads((root / "lanes" / "plan.json").read_text())
tasks = {task["task_id"]: task for task in lane_plan.get("tasks", [])}
expected = {
    "contract_brain": "ollama",
    "workflow_brain": "ollama",
    "planner_brain": "ollama",
    "repo_shape": "ollama",
    "package_contracts__ollama": "ollama",
    "package_contracts__codex": "codex",
    "package_contracts__claude": "claude",
    "safety_model__ollama": "ollama",
    "safety_model__codex": "codex",
    "safety_model__claude": "claude",
}
assert {key: tasks[key]["provider"] for key in expected} == expected, tasks
assert tasks["repo_shape"]["comparison"]["enabled"] is False, tasks["repo_shape"]
for task_id in expected:
    task = tasks[task_id]
    if task_id in {"contract_brain", "workflow_brain", "planner_brain", "repo_shape"}:
        continue
    assert task["comparison"]["enabled"] is True, task
    assert task["base_task_id"] in {"package_contracts", "safety_model"}, task
graph = json.loads((root / "task-graph.json").read_text())
graph_tasks = {task["id"]: task for task in graph["tasks"]}
for task_id, provider in expected.items():
    assert graph_tasks[task_id]["provider"] == provider, graph_tasks[task_id]
    assert graph_tasks[task_id].get("model"), graph_tasks[task_id]
    if provider == "codex":
        assert graph_tasks[task_id]["model"] == "test-codex-model", graph_tasks[task_id]
    if provider == "claude":
        assert graph_tasks[task_id]["model"] == "test-claude-model", graph_tasks[task_id]
    if provider == "ollama":
        if task_id in {"contract_brain", "workflow_brain", "repo_shape"}:
            assert graph_tasks[task_id]["model"] == "llama3.2", graph_tasks[task_id]
        else:
            assert graph_tasks[task_id]["model"] == "test-ollama-model", graph_tasks[task_id]
    prompt = (root / "tasks" / task_id / "prompt.md").read_text()
    assert "DorkPipe worker output contract:" in prompt, prompt
    assert "Return only the requested markdown artifact content." in prompt, prompt
    assert "Do not create or describe task.json" in prompt, prompt
    assert "AGENTS.md context:" in prompt, prompt
    assert "DockPipe Root Router" in prompt, prompt
    assert "Input context excerpts:" in prompt, prompt
    assert "shared/repo-map.md" in prompt, prompt
    if provider == "ollama":
        assert prompt.startswith("DorkPipe worker output contract:"), prompt
        assert "Local model lane guidance:" in prompt, prompt
assert graph["concurrency"]["max_workers"] >= 4, graph["concurrency"]
assert graph["concurrency"]["max_local_workers"] >= 2, graph["concurrency"]
assert graph["concurrency"]["max_cloud_workers"] >= 2, graph["concurrency"]
request = json.loads((root / "request.json").read_text())
assert request["compare_providers"] == ["ollama", "codex", "claude"], request
assert request["compare_scope"] == "auto", request
PY

echo "test_orchestration_compare_lanes OK"

export DORKPIPE_ORCH_WORKFLOW="test.docs.orchestrate.cloud-usage"
export DORKPIPE_ORCH_ROOT="${TMPDIR:-/tmp}/dorkpipe-orch-cloud-usage-${RANDOM}-${RANDOM}"
export DORKPIPE_ORCH_COMPARE_PROVIDERS=""

# shellcheck source=/dev/null
source "$DOCKPIPE_SCRIPT_DIR/orchestrate-common.sh"
dorkpipe_orchestrate_init
dorkpipe_orchestrate_record_cloud_usage codex 100 50 1200
dorkpipe_orchestrate_record_cloud_usage codex 25 25 800
dorkpipe_orchestrate_record_cloud_usage claude 40 10 400

python3 - "$DORKPIPE_ORCH_ROOT" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
usage = json.loads((root / "cloud-usage.json").read_text())
assert usage["cloud_task_count"] == 3, usage
assert usage["total_estimated_input_tokens"] == 165, usage
assert usage["total_estimated_output_tokens"] == 85, usage
assert usage["total_estimated_tokens"] == 250, usage
assert usage["total_duration_ms"] == 2400, usage
assert usage["providers"]["codex"]["task_count"] == 2, usage
assert usage["providers"]["codex"]["estimated_tokens"] == 200, usage
assert usage["providers"]["codex"]["duration_ms"] == 2000, usage
assert usage["providers"]["claude"]["task_count"] == 1, usage
assert usage["providers"]["claude"]["estimated_tokens"] == 50, usage
assert usage["providers"]["claude"]["duration_ms"] == 400, usage
PY

echo "test_orchestration_cloud_usage_metrics OK"
