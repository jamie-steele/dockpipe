#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:?DOCKPIPE_SCRIPT_DIR is required}"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
task_id="${1:-}"
if [[ -z "${task_id}" ]]; then
  [[ -n "${DOCKPIPE_WORKFLOW_CONFIG:-}" ]] || { echo "DOCKPIPE_WORKFLOW_CONFIG is required when task id is omitted" >&2; exit 1; }
  [[ -n "${DOCKPIPE_STEP_ID:-}" ]] || { echo "DOCKPIPE_STEP_ID is required when task id is omitted" >&2; exit 1; }
  task_id="$(
    python3 - "${DOCKPIPE_WORKFLOW_CONFIG}" "${DOCKPIPE_STEP_ID}" <<'PY'
import sys
import yaml

workflow = yaml.safe_load(open(sys.argv[1], "r", encoding="utf-8")) or {}
for step in workflow.get("steps", []):
    if isinstance(step, dict) and step.get("id") == sys.argv[2]:
        agent = step.get("agent", {}) or {}
        print(agent.get("task_id", ""))
        break
PY
  )"
fi
[[ -n "${task_id}" ]] || { echo "task id is required (argument or steps[].agent.task_id)" >&2; exit 1; }
task_dir="$(dorkpipe_orchestrate_task_dir "${task_id}")"
prompt_md="${task_dir}/prompt.md"
response_md="${task_dir}/response.md"
result_json="${task_dir}/result.json"

[[ -f "${task_dir}/task.json" ]] || { echo "missing task.json for ${task_id}" >&2; exit 1; }
eval "$(
  python3 - "${task_dir}/task.json" <<'PY'
import json
import shlex
import sys

task = json.load(open(sys.argv[1], "r", encoding="utf-8"))
mapping = {
    "TASK_RESOLVER_HINT": task.get("resolver_hint", "auto"),
    "TASK_REQUESTED_RESOLVER_HINT": task.get("requested_resolver_hint", task.get("resolver_hint", "auto")),
    "TASK_LANE_JSON": json.dumps(task.get("lane", {})),
    "TASK_LANE_ID": (task.get("lane", {}) or {}).get("lane_id", ""),
    "TASK_GOAL": task.get("goal", ""),
    "TASK_EXPECTED_OUTPUT": task.get("expected_output", ""),
    "TASK_INPUTS_JSON": json.dumps(task.get("inputs", [])),
    "TASK_CLAIMS_JSON": json.dumps(task.get("claims", [])),
    "TASK_CITATIONS_JSON": json.dumps(task.get("citations", task.get("inputs", []))),
    "TASK_MAX_CLOUD_TOKENS": str(task.get("max_cloud_tokens", "")),
    "TASK_MODEL_JSON": json.dumps(task.get("model", {})),
}
for key, value in mapping.items():
    print(f"{key}={shlex.quote(value)}")
PY
)"
resolver_hint="${TASK_RESOLVER_HINT:-auto}"
provider="$(dorkpipe_orchestrate_resolve_provider "${resolver_hint:-auto}")"
lane_id="${TASK_LANE_ID:-${provider}}"
used_live_model="false"
status="ok"
summary="Fallback worker output for ${task_id}"
confidence="0.55"
issues_json='["live worker backend unavailable for this task"]'
next_actions_json='["review merged output before treating it as final"]'
budget_halt="false"
estimated_input_tokens="$(dorkpipe_orchestrate_estimate_tokens_for_file "${prompt_md}")"
estimated_output_tokens="0"
estimated_total_tokens="${estimated_input_tokens}"

live_response_is_valid() {
  local path="${1:?response path}"
  [[ -s "${path}" ]] || return 1
  python3 - "${path}" <<'PY'
import pathlib
import re
import sys

text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8", errors="replace").strip()
if len(text) < 40:
    raise SystemExit(1)
if re.fullmatch(r"sha256:[0-9a-f]{64}", text):
    raise SystemExit(1)
if re.search(r"\b(exec|command): .* not found\b", text, flags=re.IGNORECASE):
    raise SystemExit(1)
PY
}

if dorkpipe_orchestrate_is_cloud_provider "${provider}"; then
  if [[ -f "${DORKPIPE_ORCH_HALT_JSON}" ]]; then
    budget_halt="true"
    status="skipped"
    summary="Skipped live ${provider} worker because the orchestration run was already halted."
    confidence="0.20"
    issues_json='["cloud budget halt was already active before this task started"]'
    next_actions_json='["review cloud-usage.json and halt.json before resuming cloud workers"]'
  elif (( estimated_input_tokens > ${TASK_MAX_CLOUD_TOKENS:-$DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS} )); then
    budget_halt="true"
    status="skipped"
    summary="Skipped live ${provider} worker because the prompt estimate exceeded the per-task cloud token budget."
    confidence="0.20"
    issues_json='["estimated prompt tokens exceeded the per-task cloud token budget"]'
    next_actions_json='["shrink the task scope or raise DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS intentionally"]'
    dorkpipe_orchestrate_halt_run "${provider}" "Prompt estimate for ${task_id} exceeded the per-task cloud token budget (${estimated_input_tokens}/${TASK_MAX_CLOUD_TOKENS:-$DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS})."
  elif (( $(sed -n 's/.*"total_estimated_tokens": \([0-9][0-9]*\).*/\1/p' "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" | head -1 || echo 0) + estimated_input_tokens > DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS )) && [[ "$(dorkpipe_orchestrate_bool "${DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED}")" == "true" ]]; then
    budget_halt="true"
    status="skipped"
    summary="Skipped live ${provider} worker because starting it would exceed the orchestration cloud token budget."
    confidence="0.20"
    issues_json='["estimated prompt tokens would exceed the total cloud token budget before the task started"]'
    next_actions_json='["review cloud-usage.json and either reduce scope or raise the total cloud budget intentionally"]'
    dorkpipe_orchestrate_halt_run "${provider}" "Starting ${task_id} would exceed the total cloud token budget."
  fi
fi

if [[ "${budget_halt}" != "true" ]]; then
  if [[ "$(dorkpipe_orchestrate_bool "${DORKPIPE_ORCH_LIVE_MODELS}")" != "true" ]]; then
    issues_json='["live model execution disabled by DORKPIPE_ORCH_LIVE_MODELS"]'
  else
    case "${provider}" in
      codex)
        if [[ "$(dorkpipe_orchestrate_bool "${DORKPIPE_ORCH_CONTAINERIZE_CLOUD}")" == "true" ]]; then
          if dorkpipe_orchestrate_run_container_worker codex "${prompt_md}" "${response_md}" && live_response_is_valid "${response_md}"; then
            used_live_model="true"
            summary="Live Codex worker output captured in response.md from the codex resolver container"
            confidence="0.72"
            issues_json='[]'
            next_actions_json='["merge this task with sibling worker outputs"]'
          else
            issues_json='["codex resolver container failed, host Codex auth was unavailable, or output was not a model response"]'
          fi
        elif command -v codex >/dev/null 2>&1; then
          if codex exec --dangerously-bypass-approvals-and-sandbox "$(cat "${prompt_md}")" > "${response_md}" && live_response_is_valid "${response_md}"; then
            used_live_model="true"
            summary="Live Codex worker output captured in response.md from the host CLI"
            confidence="0.72"
            issues_json='[]'
            next_actions_json='["merge this task with sibling worker outputs"]'
          fi
        else
          issues_json='["codex CLI is not installed or not available on PATH"]'
        fi
        ;;
      claude)
        if [[ "$(dorkpipe_orchestrate_bool "${DORKPIPE_ORCH_CONTAINERIZE_CLOUD}")" == "true" ]]; then
          if dorkpipe_orchestrate_run_container_worker claude "${prompt_md}" "${response_md}" && live_response_is_valid "${response_md}"; then
            used_live_model="true"
            summary="Live Claude worker output captured in response.md from the claude resolver container"
            confidence="0.72"
            issues_json='[]'
            next_actions_json='["merge this task with sibling worker outputs"]'
          else
            issues_json='["claude resolver container failed, host Claude auth was unavailable, or output was not a model response"]'
          fi
        elif command -v claude >/dev/null 2>&1; then
          if claude -p "$(cat "${prompt_md}")" > "${response_md}" && live_response_is_valid "${response_md}"; then
            used_live_model="true"
            summary="Live Claude worker output captured in response.md from the host CLI"
            confidence="0.72"
            issues_json='[]'
            next_actions_json='["merge this task with sibling worker outputs"]'
          fi
        else
          issues_json='["claude CLI is not installed or not available on PATH"]'
        fi
        ;;
      ollama)
        if command -v curl >/dev/null 2>&1 && command -v jq >/dev/null 2>&1; then
          ollama_host="${OLLAMA_HOST:-http://host.docker.internal:11434}"
          ollama_host="${ollama_host%/}"
          ollama_model="$(
            python3 - <<'PY'
import json
import os

model = ""
try:
    model = (json.loads(os.environ.get("TASK_MODEL_JSON", "{}")) or {}).get("model", "") or ""
except Exception:
    model = ""
print(model)
PY
          )"
          if [[ -z "${ollama_model}" ]]; then
            ollama_model="${DORKPIPE_ORCH_OLLAMA_MODEL:-llama3.2}"
          fi
          req_json="$(jq -n --arg model "${ollama_model}" --arg prompt "$(cat "${prompt_md}")" '{model:$model,messages:[{role:"user",content:$prompt}],stream:false}')"
          if resp_json="$(curl -sf "${ollama_host}/api/chat" -H 'Content-Type: application/json' -d "${req_json}")"; then
            if printf '%s' "${resp_json}" | jq -er '.message.content' > "${response_md}"; then
              used_live_model="true"
              summary="Live Ollama worker output captured in response.md"
              confidence="0.68"
              issues_json='[]'
              next_actions_json='["merge this task with sibling worker outputs"]'
            fi
          fi
        fi
        ;;
    esac
  fi
fi

if [[ "${used_live_model}" != "true" ]]; then
  cat > "${response_md}" <<EOF
# ${task_id}

Fallback worker output for provider \`${provider}\`.

- Goal: ${TASK_GOAL}
- Expected output: ${TASK_EXPECTED_OUTPUT}
- Task stayed bounded and artifact-driven.
EOF
fi

estimated_output_tokens="$(dorkpipe_orchestrate_estimate_tokens_for_file "${response_md}")"
estimated_total_tokens="$(( estimated_input_tokens + estimated_output_tokens ))"

if [[ "${used_live_model}" == "true" ]] && dorkpipe_orchestrate_is_cloud_provider "${provider}"; then
  dorkpipe_orchestrate_record_cloud_usage "${provider}" "${estimated_input_tokens}" "${estimated_output_tokens}"
fi

dorkpipe_orchestrate_record_training_metric "${task_id}" "${lane_id}" "${provider}" "${status}" "${confidence}" "${estimated_input_tokens}" "${estimated_output_tokens}" "${used_live_model}" "${budget_halt}"

export task_id status resolver_hint provider lane_id used_live_model budget_halt estimated_input_tokens estimated_output_tokens estimated_total_tokens summary confidence issues_json next_actions_json TASK_LANE_JSON TASK_CLAIMS_JSON TASK_CITATIONS_JSON
python3 - "${result_json}" <<'PY'
import json
import os
import sys

def loads_env(name, fallback):
    try:
        return json.loads(os.environ.get(name, ""))
    except Exception:
        return fallback

payload = {
    "task_id": os.environ["task_id"],
    "status": os.environ["status"],
    "provider_requested": os.environ.get("resolver_hint", "auto"),
    "provider_actual": os.environ["provider"],
    "lane_id": os.environ.get("lane_id", ""),
    "lane_selection": loads_env("TASK_LANE_JSON", {}),
    "used_live_model": os.environ.get("used_live_model") == "true",
    "budget_halt": os.environ.get("budget_halt") == "true",
    "estimated_input_tokens": int(os.environ.get("estimated_input_tokens", "0")),
    "estimated_output_tokens": int(os.environ.get("estimated_output_tokens", "0")),
    "estimated_total_tokens": int(os.environ.get("estimated_total_tokens", "0")),
    "summary": os.environ.get("summary", ""),
    "claims": loads_env("TASK_CLAIMS_JSON", []),
    "artifacts": [
        f"tasks/{os.environ['task_id']}/task.json",
        f"tasks/{os.environ['task_id']}/prompt.md",
        f"tasks/{os.environ['task_id']}/response.md",
    ],
    "citations": loads_env("TASK_CITATIONS_JSON", []),
    "confidence": float(os.environ.get("confidence", "0")),
    "issues": loads_env("issues_json", []),
    "next_actions": loads_env("next_actions_json", []),
}
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps(payload, indent=2) + "\n")
PY

printf '[dorkpipe] task %s result ready at %s\n' "${task_id}" "${result_json}" >&2
