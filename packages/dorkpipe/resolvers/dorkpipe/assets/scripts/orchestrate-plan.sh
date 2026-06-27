#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:?DOCKPIPE_SCRIPT_DIR is required}"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
rm -f "${DORKPIPE_ORCH_HALT_JSON}"
cat > "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" <<EOF
{
  "max_total_cloud_tokens": ${DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS},
  "max_task_cloud_tokens": ${DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS},
  "stop_on_budget_exceeded": ${DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED},
  "total_estimated_input_tokens": 0,
  "total_estimated_output_tokens": 0,
  "total_estimated_tokens": 0,
  "total_duration_ms": 0,
  "cloud_task_count": 0,
  "budget_exceeded": false,
  "halted": false,
  "providers": {
    "codex": {"task_count": 0, "estimated_tokens": 0, "duration_ms": 0},
    "claude": {"task_count": 0, "estimated_tokens": 0, "duration_ms": 0}
  }
}
EOF
[[ -n "${DOCKPIPE_WORKFLOW_CONFIG:-}" ]] || {
  echo "DOCKPIPE_WORKFLOW_CONFIG is required" >&2
  exit 1
}
[[ -f "${DOCKPIPE_WORKFLOW_CONFIG}" ]] || {
  echo "missing workflow config: ${DOCKPIPE_WORKFLOW_CONFIG}" >&2
  exit 1
}
[[ -n "${DOCKPIPE_STEP_ID:-}" ]] || {
  echo "DOCKPIPE_STEP_ID is required for orchestration planning" >&2
  exit 1
}

python3 - "${DOCKPIPE_WORKFLOW_CONFIG}" "${DOCKPIPE_STEP_ID}" <<'PY'
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys

import yaml

workflow_config = pathlib.Path(sys.argv[1])
step_id = sys.argv[2]
root = pathlib.Path(os.environ["ROOT"])
shared_dir = pathlib.Path(os.environ["DORKPIPE_ORCH_SHARED_DIR"])
tasks_dir = pathlib.Path(os.environ["DORKPIPE_ORCH_TASKS_DIR"])
request_json = pathlib.Path(os.environ["DORKPIPE_ORCH_REQUEST_JSON"])
plan_json = pathlib.Path(os.environ["DORKPIPE_ORCH_PLAN_JSON"])
graph_json = pathlib.Path(os.environ["DORKPIPE_ORCH_GRAPH_JSON"])
lane_plan_json = pathlib.Path(os.environ["DORKPIPE_ORCH_LANE_PLAN_JSON"])
model_catalog_path = pathlib.Path(os.environ["DORKPIPE_ORCH_MODEL_CATALOG"])
baseline_policy_path = pathlib.Path(os.environ["DORKPIPE_ORCH_BASELINE_POLICY"])
global_training_metrics_path = pathlib.Path(os.environ["DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS"])
workflow_name = os.environ["DORKPIPE_ORCH_WORKFLOW"]
artifact_root = os.environ["DORKPIPE_ORCH_ROOT"]
max_total_cloud_tokens = int(os.environ["DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS"])
max_task_cloud_tokens = int(os.environ["DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS"])
stop_on_budget_exceeded = os.environ["DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED"].lower() in {"1", "true", "yes", "on"}
training_mode = os.environ.get("DORKPIPE_ORCH_TRAINING_MODE", "observe")
cloud_lanes_enabled = os.environ.get("DORKPIPE_ORCH_CLOUD_LANES", "false").lower() in {"1", "true", "yes", "on"}
force_provider = (os.environ.get("DORKPIPE_ORCH_FORCE_PROVIDER") or os.environ.get("DORKPIPE_ORCH_TASK_PROVIDER") or "").strip().lower()
force_provider_scope = (os.environ.get("DORKPIPE_ORCH_FORCE_PROVIDER_SCOPE") or "auto").strip().lower()
compare_providers = [
    item.strip().lower()
    for item in (os.environ.get("DORKPIPE_ORCH_COMPARE_PROVIDERS") or "").split(",")
    if item.strip()
]
compare_scope = (os.environ.get("DORKPIPE_ORCH_COMPARE_SCOPE") or "auto").strip().lower()

workflow = yaml.safe_load(workflow_config.read_text()) or {}
workflow_model_policy = workflow.get("model_policy", {}) or {}
steps = workflow.get("steps", [])
current_step = None
for step in steps:
    if isinstance(step, dict) and step.get("id") == step_id:
        current_step = step
        break
if current_step is None:
    raise SystemExit(f"{workflow_config}: could not find step id '{step_id}'")

agent = current_step.get("agent", {}) or {}
orchestration = agent.get("orchestration", {}) or {}
request = orchestration.get("request", {})
plan = orchestration.get("plan", {})
shared = orchestration.get("shared", [])
tasks = orchestration.get("tasks", [])
merge = orchestration.get("merge", {})
verify = orchestration.get("verify", {})
concurrency = orchestration.get("concurrency", {}) or {}
apply = orchestration.get("apply", {}) or {}
startup_prompt = agent.get("startup_prompt", "")
include_agents_md = bool(agent.get("include_agents_md"))
workflow_accessible_paths = agent.get("accessible_paths", [])
workflow_access = agent.get("access", {}) or {}
agent_model_policy = agent.get("model_policy", {}) or workflow_model_policy

if not tasks:
    raise SystemExit(f"{workflow_config}: steps[].agent.orchestration.tasks must contain at least one task")

def expand_env(value):
    text = str(value or "")
    pattern = re.compile(r"\$\{([^}:]+)(:-([^}]*))?\}")
    def replace(match):
        key = match.group(1)
        default = match.group(3) or ""
        return os.environ.get(key, default)
    return pattern.sub(replace, text)

def load_model_lanes():
    if not model_catalog_path.exists():
        return []
    raw = yaml.safe_load(model_catalog_path.read_text()) or {}
    lanes = raw.get("lanes", []) if isinstance(raw, dict) else []
    normalized = []
    for lane in lanes:
        if not isinstance(lane, dict) or not lane.get("id"):
            continue
        item = dict(lane)
        item["model"] = expand_env(item.get("model", ""))
        commands = ((item.get("availability") or {}).get("commands") or [])
        missing = [cmd for cmd in commands if not shutil.which(str(cmd))]
        item["available"] = not missing
        item["missing_commands"] = missing
        normalized.append(item)
    return normalized

model_lanes = load_model_lanes()

def load_baseline_policy():
    if not baseline_policy_path.exists():
        return {}
    return yaml.safe_load(baseline_policy_path.read_text()) or {}

baseline_policy = load_baseline_policy()
selection_policy = baseline_policy.get("selection", {}) or {}
training_policy = baseline_policy.get("training", {}) or {}

def words_in_text(text, words):
    return any(re.search(rf"\b{re.escape(str(word).lower())}\b", text) for word in words or [])

def load_training_stats():
    stats = {}
    if not global_training_metrics_path.exists():
        return stats
    for line in global_training_metrics_path.read_text().splitlines():
        if not line.strip():
            continue
        try:
            metric = json.loads(line)
        except Exception:
            continue
        lane_id = metric.get("lane_id") or metric.get("provider")
        if not lane_id:
            continue
        entry = stats.setdefault(lane_id, {
            "samples": 0,
            "confidence_total": 0.0,
            "live_successes": 0,
            "budget_halts": 0,
        })
        entry["samples"] += 1
        entry["confidence_total"] += float(metric.get("confidence") or 0)
        if metric.get("used_live_model") and metric.get("status") == "ok":
            entry["live_successes"] += 1
        if metric.get("budget_halt"):
            entry["budget_halts"] += 1
    for entry in stats.values():
        samples = max(1, entry["samples"])
        entry["avg_confidence"] = entry["confidence_total"] / samples
        entry["live_success_rate"] = entry["live_successes"] / samples
        entry["budget_halt_rate"] = entry["budget_halts"] / samples
    return stats

training_stats = load_training_stats()

def training_adjustment(lane_id):
    entry = training_stats.get(lane_id) or {}
    samples = int(entry.get("samples") or 0)
    min_samples = int(training_policy.get("min_samples_before_adjustment") or 20)
    if samples < min_samples:
        return 0.0, {"samples": samples, "active": False}
    target = float(training_policy.get("target_confidence") or 0.72)
    weight = float(training_policy.get("score_adjustment_weight") or 6.0)
    cap = float(training_policy.get("max_score_adjustment") or 4.0)
    avg_conf = float(entry.get("avg_confidence") or 0)
    budget_halt_rate = float(entry.get("budget_halt_rate") or 0)
    adjustment = (avg_conf - target) * weight - (budget_halt_rate * weight)
    adjustment = max(-cap, min(cap, adjustment))
    detail = dict(entry)
    detail["active"] = True
    detail["adjustment"] = adjustment
    return adjustment, detail

def lane_score(lane, task, policy, requested):
    score = 0.0
    reason = []
    provider = lane.get("provider", "")
    resolver_hint = lane.get("resolver_hint", provider)
    capabilities = set(lane.get("capabilities", []) or [])
    text = " ".join([
        str(task.get("goal", "")),
        str(task.get("expected_output", "")),
        " ".join(task.get("constraints", []) or []),
        str(task.get("worker_type", "")),
    ]).lower()
    if requested and requested != "auto":
        if requested in {provider, resolver_hint, lane.get("id")}:
            score += 100
            reason.append(f"explicit resolver_hint matched {requested}")
        else:
            score -= 100
    attempt_pref = ((policy.get("attempt") or {}).get("preference") or "").lower()
    validate_pref = ((policy.get("validate") or {}).get("preference") or "").lower()
    if attempt_pref in {"local", "local-first", "cheap", "cheap-first"} and lane.get("local"):
        score += float(selection_policy.get("local_first_bonus", 15.0))
        reason.append("local/cheap attempt preference")
    if lane.get("cloud"):
        score -= float(selection_policy.get("cloud_cost_penalty", 2.0))
    if validate_pref in {"strong", "strongest", "strongest_available"} and "strong_validation" in capabilities:
        score += float(selection_policy.get("strong_validation_bonus", 8.0))
        reason.append("strong validation capability")
    if words_in_text(text, selection_policy.get("code_keywords", ["patch", "code", "implementation", "edit"])) and "code" in capabilities:
        score += float(selection_policy.get("code_task_bonus", 4.0))
        reason.append("code task capability")
    if words_in_text(text, selection_policy.get("safety_keywords", ["safety", "approval", "risk", "security", "review"])) and ("safety" in capabilities or "review" in capabilities):
        score += float(selection_policy.get("safety_review_bonus", 4.0))
        reason.append("review/safety capability")
    if not lane.get("available"):
        score -= float(selection_policy.get("unavailable_penalty", 25.0))
        reason.append("lane availability check failed")
    score += float(((lane.get("training") or {}).get("exploration_weight") or 0))
    adjustment, training_detail = training_adjustment(lane.get("id", provider))
    if adjustment:
        score += adjustment
        reason.append(f"historical training adjustment {adjustment:+.2f}")
    return score, reason, training_detail

def high_risk_task(task):
    text = " ".join([
        str(task.get("id", "")),
        str(task.get("goal", "")),
        str(task.get("expected_output", "")),
        " ".join(task.get("constraints", []) or []),
        str(task.get("worker_type", "")),
    ]).lower()
    return words_in_text(text, selection_policy.get("high_risk_keywords", []))

def select_lane(task, policy, requested_override=""):
    requested = str(task.get("resolver_hint", "auto") or "auto")
    if requested_override:
        requested = requested_override
    elif force_provider and (force_provider_scope == "all" or requested == "auto"):
        requested = force_provider
    candidates = []
    for lane in model_lanes:
        score, reason, training_detail = lane_score(lane, task, policy, requested)
        candidates.append({
            "lane": lane,
            "score": score,
            "reason": reason,
            "training": training_detail,
        })
    candidates.sort(key=lambda item: item["score"], reverse=True)
    selected = candidates[0] if candidates else {
        "lane": {
            "id": requested if requested != "auto" else "ollama.local.default",
            "provider": requested if requested != "auto" else "ollama",
            "resolver_hint": requested if requested != "auto" else "ollama",
            "model": (task.get("model") or {}).get("model", ""),
            "cloud": requested in {"codex", "claude"},
            "available": False,
        },
        "score": 0,
        "reason": ["fallback lane because catalog is unavailable"],
        "training": {"active": False, "samples": 0},
    }
    gated_by_baseline = False
    baseline_gate_reason = ""
    explicit_hint = requested and requested != "auto"
    if selected["lane"].get("cloud") and not cloud_lanes_enabled:
        bypass = explicit_hint and bool(selection_policy.get("explicit_hint_bypasses_cloud_gate", True))
        if not bypass:
            local_candidates = [item for item in candidates if (item.get("lane") or {}).get("local") and (item.get("lane") or {}).get("available")]
            if local_candidates:
                baseline_gate_reason = "cloud lane gated because DORKPIPE_ORCH_CLOUD_LANES=false"
                selected = local_candidates[0]
                gated_by_baseline = True
    if selected["lane"].get("cloud") and cloud_lanes_enabled:
        threshold = float(selection_policy.get("high_risk_cloud_score_threshold" if high_risk_task(task) else "cloud_score_threshold", 14.0))
        if float(selected.get("score") or 0) < threshold:
            local_candidates = [item for item in candidates if (item.get("lane") or {}).get("local") and (item.get("lane") or {}).get("available")]
            if local_candidates:
                baseline_gate_reason = f"cloud lane gated by baseline threshold {threshold:.1f}"
                selected = local_candidates[0]
                gated_by_baseline = True
    lane = dict(selected["lane"])
    task_model = task.get("model") or {}
    if task_model.get("model"):
        lane["model"] = expand_env(task_model.get("model"))
    return {
        "task_id": task.get("id", ""),
        "requested": requested,
        "lane_id": lane.get("id", ""),
        "provider": lane.get("provider", lane.get("resolver_hint", requested)),
        "resolver_hint": lane.get("resolver_hint", lane.get("provider", requested)),
        "model": lane.get("model", ""),
        "cloud": bool(lane.get("cloud")),
        "local": bool(lane.get("local")),
        "available": bool(lane.get("available")),
        "missing_commands": lane.get("missing_commands", []),
        "capabilities": lane.get("capabilities", []),
        "context_window": int(lane.get("context_window", 0) or 0),
        "max_task_tokens": int(((lane.get("budget") or {}).get("max_task_tokens") or max_task_cloud_tokens)),
        "score": selected["score"],
        "reasons": selected["reason"],
        "gated_by_baseline": gated_by_baseline,
        "baseline_gate_reason": baseline_gate_reason,
        "training": selected.get("training", {}),
    }

def comparison_enabled_for_task(task):
    if not compare_providers:
        return False
    requested = str(task.get("resolver_hint", "auto") or "auto")
    if compare_scope == "all":
        return True
    return requested == "auto"

def comparison_task_id(task_id, provider):
    safe = re.sub(r"[^A-Za-z0-9_]+", "_", str(provider)).strip("_") or "lane"
    return f"{task_id}__{safe}"

def run_text(*args: str) -> str:
    try:
        return subprocess.check_output(args, cwd=str(root), text=True, stderr=subprocess.STDOUT)
    except subprocess.CalledProcessError as exc:
        return exc.output or ""

def render_shared(entry: dict) -> str:
    collector = entry.get("collector", "literal")
    if collector == "literal":
        return entry.get("text", "")
    if collector == "repo_map":
        tracked = run_text("git", "-C", str(root), "ls-files")
        tracked_count = len([line for line in tracked.splitlines() if line.strip()])
        lines = ["# Repo Map", "", f"- Tracked files: {tracked_count}"]
        if entry.get("focus"):
            lines.append(f"- Focus: {entry['focus']}")
        paths = entry.get("paths", [])
        if paths:
            lines.extend(["", "## Key Paths", ""])
            for rel in paths:
                if (root / rel).exists():
                    lines.append(f"- `{rel}`")
        return "\n".join(lines) + "\n"
    if collector == "dockpipe_cli_inspect":
        dockpipe_bin = root / "src/bin/dockpipe"
        dockpipe_cmd = str(dockpipe_bin) if dockpipe_bin.exists() else "dockpipe"
        version = run_text(dockpipe_cmd, "--version").strip()
        packages = run_text(dockpipe_cmd, "package", "list", "--workdir", str(root)).rstrip()
        return "\n".join([
            "# CLI Inspect",
            "",
            "```text",
            version,
            "```",
            "",
            "## Package List",
            "",
            "```text",
            packages,
            "```",
            "",
        ])
    raise SystemExit(f"{workflow_config}: unknown shared collector '{collector}'")

for entry in shared:
    rel = entry.get("path")
    if not rel:
        raise SystemExit(f"{workflow_config}: each shared entry needs path:")
    dest = shared_dir / rel
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(render_shared(entry))

request_payload = {
    "contract_version": "v1",
    "workflow": workflow_name,
    "request": request.get("text", ""),
    "artifact_root": artifact_root,
    "workflow_config": str(workflow_config),
    "step_id": step_id,
    "cloud_budget": {
        "max_total_cloud_tokens": max_total_cloud_tokens,
        "max_task_cloud_tokens": max_task_cloud_tokens,
        "stop_on_budget_exceeded": stop_on_budget_exceeded,
    },
    "access": workflow_access,
    "model_policy": agent_model_policy,
    "model_catalog": str(model_catalog_path),
    "training_mode": training_mode,
    "force_provider": force_provider,
    "force_provider_scope": force_provider_scope,
    "compare_providers": compare_providers,
    "compare_scope": compare_scope,
}
request_json.write_text(json.dumps(request_payload, indent=2) + "\n")

plan_payload = {
    "goal": plan.get("goal", request.get("text", "")),
    "steps": plan.get("steps", []),
    "cloud_budget": request_payload["cloud_budget"],
    "concurrency": {
        "max_workers": int(concurrency.get("max_workers", 1) or 1),
        "max_local_workers": int(concurrency.get("max_local_workers", concurrency.get("max_workers", 1)) or 1),
        "max_cloud_workers": int(concurrency.get("max_cloud_workers", 1) or 1),
    },
    "merge": {
        "title": merge.get("title", "DorkPipe Orchestration Synthesis"),
        "summary_points": merge.get("summary_points", []),
    },
    "verify": {
        "next_action_default": verify.get("next_action_default", "human approval before treating orchestration output as final"),
    },
    "apply": {
        "require_approval": bool(apply.get("require_approval", True)),
        "outputs": apply.get("outputs", []),
    },
}
if compare_providers:
    compare_width = len(compare_providers)
    plan_payload["concurrency"]["max_workers"] = max(plan_payload["concurrency"]["max_workers"], compare_width)
    plan_payload["concurrency"]["max_cloud_workers"] = max(plan_payload["concurrency"]["max_cloud_workers"], compare_width)
plan_json.write_text(json.dumps(plan_payload, indent=2) + "\n")

graph_tasks = []
worker_ids = []
lane_plan = {
    "catalog": str(model_catalog_path),
    "baseline_policy": str(baseline_policy_path),
    "training_mode": training_mode,
    "force_provider": force_provider,
    "force_provider_scope": force_provider_scope,
    "compare_providers": compare_providers,
    "compare_scope": compare_scope,
    "cloud_lanes_enabled": cloud_lanes_enabled,
    "global_training_metrics": str(global_training_metrics_path),
    "policy": agent_model_policy,
    "thresholds": {
        "cloud_score_threshold": float(selection_policy.get("cloud_score_threshold", 14.0)),
        "high_risk_cloud_score_threshold": float(selection_policy.get("high_risk_cloud_score_threshold", 10.0)),
        "min_samples_before_adjustment": int(training_policy.get("min_samples_before_adjustment") or 20),
    },
    "lanes": [
        {
            "id": lane.get("id"),
            "provider": lane.get("provider"),
            "resolver_hint": lane.get("resolver_hint"),
            "model": lane.get("model"),
            "local": bool(lane.get("local")),
            "cloud": bool(lane.get("cloud")),
            "available": bool(lane.get("available")),
            "missing_commands": lane.get("missing_commands", []),
            "capabilities": lane.get("capabilities", []),
            "context_window": int(lane.get("context_window", 0) or 0),
        }
        for lane in model_lanes
    ],
    "tasks": [],
}

for task in tasks:
    base_task_id = task.get("id")
    if not base_task_id:
        raise SystemExit(f"{workflow_config}: each task needs id:")

    if comparison_enabled_for_task(task):
        task_variants = [
            {
                "task_id": comparison_task_id(base_task_id, provider),
                "base_task_id": base_task_id,
                "compare_provider": provider,
                "requested_override": provider,
            }
            for provider in compare_providers
        ]
    else:
        task_variants = [{
            "task_id": base_task_id,
            "base_task_id": base_task_id,
            "compare_provider": "",
            "requested_override": "",
        }]

    for variant in task_variants:
        task_id = variant["task_id"]
        worker_ids.append(task_id)

        task_dir = tasks_dir / task_id
        task_dir.mkdir(parents=True, exist_ok=True)
        task_model = task.get("model", {}) or agent.get("model", {}) or {}
        task_policy = task.get("model_policy", agent_model_policy)
        lane_selection = select_lane(task, task_policy, variant["requested_override"])
        lane_selection["task_id"] = task_id
        lane_selection["base_task_id"] = variant["base_task_id"]
        if variant["compare_provider"]:
            lane_selection["comparison"] = {
                "enabled": True,
                "base_task_id": variant["base_task_id"],
                "provider": variant["compare_provider"],
                "providers": compare_providers,
            }
        else:
            lane_selection["comparison"] = {"enabled": False}
        (task_dir / "lane-selection.json").write_text(json.dumps(lane_selection, indent=2) + "\n")
        lane_plan["tasks"].append(lane_selection)

        accessible_paths = list(workflow_accessible_paths)
        for path in task.get("accessible_paths", []) or []:
            if path not in accessible_paths:
                accessible_paths.append(path)
        task_access = {
            "read": list(workflow_access.get("read", []) or []),
            "write": list(workflow_access.get("write", []) or []),
            "deny": list(workflow_access.get("deny", []) or []),
        }
        for mode in ("read", "write", "deny"):
            for path in (task.get("access", {}) or {}).get(mode, []) or []:
                if path not in task_access[mode]:
                    task_access[mode].append(path)

        task_payload = {
            "id": task_id,
            "base_id": variant["base_task_id"],
            "comparison": lane_selection.get("comparison", {"enabled": False}),
            "goal": task.get("goal", ""),
            "inputs": task.get("inputs", []),
            "constraints": task.get("constraints", []),
            "expected_output": task.get("expected_output", ""),
            "worker_type": task.get("worker_type", "analysis"),
            "resolver_hint": lane_selection.get("resolver_hint") or task.get("resolver_hint", "auto"),
            "requested_resolver_hint": task.get("resolver_hint", "auto"),
            "lane": lane_selection,
            "max_cloud_tokens": int(task.get("max_cloud_tokens", lane_selection.get("max_task_tokens") or max_task_cloud_tokens)),
            "depends_on": [
                comparison_task_id(dep, variant["compare_provider"])
                if variant["compare_provider"] and any(t.get("id") == dep and comparison_enabled_for_task(t) for t in tasks)
                else dep
                for dep in task.get("depends_on", [])
            ],
            "claims": task.get("claims", []),
            "citations": task.get("citations", task.get("inputs", [])),
            "startup_prompt": startup_prompt,
            "include_agents_md": include_agents_md,
            "accessible_paths": accessible_paths,
            "access": task_access,
            "model": task_model or {
                "provider": lane_selection.get("provider", ""),
                "model": lane_selection.get("model", ""),
                "num_ctx": lane_selection.get("context_window", 0),
            },
            "model_policy": task_policy,
        }
        (task_dir / "task.json").write_text(json.dumps(task_payload, indent=2) + "\n")

        prompt = task.get("prompt")
        if not prompt:
            lines = [
                "You are one worker in a DorkPipe orchestration graph.",
                "",
                f"Task id: {task_id}",
                f"Base task id: {variant['base_task_id']}",
                f"Goal: {task_payload['goal']}",
                f"Expected output: {task_payload['expected_output']}",
                "",
                "Rules:",
                "- Treat this as one bounded task, not the whole request.",
                "- Ground claims in the referenced inputs.",
                "- Return concise markdown suitable for downstream merge.",
                "- Return the requested artifact content directly; do not narrate your tool workflow.",
                "- Call out uncertainty explicitly.",
            ]
            if variant["compare_provider"]:
                lines.extend([
                    "",
                    "Comparison mode:",
                    f"- You are the {variant['compare_provider']} fork for base task `{variant['base_task_id']}`.",
                    "- Produce an independent answer for later side-by-side evaluation.",
                ])
            prompt = "\n".join(lines) + "\n"

        prefix = []
        if startup_prompt:
            prefix.append(startup_prompt.rstrip())
        if accessible_paths:
            prefix.extend(["", "Accessible paths:", *[f"- {path}" for path in accessible_paths]])
        if any(task_access.values()):
            prefix.extend(["", "Access policy:"])
            for mode in ("read", "write", "deny"):
                if task_access[mode]:
                    prefix.extend([f"{mode}:", *[f"- {path}" for path in task_access[mode]]])
        if include_agents_md and (root / "AGENTS.md").exists():
            prefix.extend(["", "AGENTS.md context:", "", (root / "AGENTS.md").read_text().rstrip()])
        if prefix:
            prompt = "\n".join(prefix).rstrip() + "\n\n" + prompt.lstrip()
        (task_dir / "prompt.md").write_text(prompt)

        graph_tasks.append({
            "id": task_id,
            "depends_on": task_payload["depends_on"],
            "resolver_hint": task_payload["resolver_hint"],
            "lane_id": lane_selection.get("lane_id", ""),
            "provider": lane_selection.get("provider", ""),
            "worker_type": task_payload["worker_type"],
        })

merge_id = merge.get("id", "merge_final")
verify_id = verify.get("id", "verify_final")
graph_tasks.append({
    "id": merge_id,
    "depends_on": worker_ids,
    "worker_type": "merge",
})
graph_tasks.append({
    "id": verify_id,
    "depends_on": [merge_id],
    "worker_type": "verify",
})
graph_json.write_text(json.dumps({"concurrency": plan_payload["concurrency"], "tasks": graph_tasks}, indent=2) + "\n")
lane_plan_json.parent.mkdir(parents=True, exist_ok=True)
lane_plan_json.write_text(json.dumps(lane_plan, indent=2) + "\n")
PY

printf '[dorkpipe] orchestration plan ready at %s\n' "${DORKPIPE_ORCH_ROOT}" >&2
