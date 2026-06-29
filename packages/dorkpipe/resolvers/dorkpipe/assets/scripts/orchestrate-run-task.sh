#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
task_started_at_ms="$(dorkpipe_orchestrate_now_ms)"
task_started_at="$(dorkpipe_orchestrate_now_iso)"
task_id="${1:-}"
if [[ -z "${task_id}" ]]; then
  [[ -n "${DOCKPIPE_WORKFLOW_CONFIG:-}" ]] || { echo "DOCKPIPE_WORKFLOW_CONFIG is required when task id is omitted" >&2; exit 1; }
  [[ -n "${DOCKPIPE_STEP_ID:-}" ]] || { echo "DOCKPIPE_STEP_ID is required when task id is omitted" >&2; exit 1; }
  task_id="$("$(dorkpipe_orchestrate_helper_bin)" task-id-from-workflow "${DOCKPIPE_WORKFLOW_CONFIG}" "${DOCKPIPE_STEP_ID}")"
fi
[[ -n "${task_id}" ]] || { echo "task id is required (argument or steps[].agent.task_id)" >&2; exit 1; }
task_dir="$(dorkpipe_orchestrate_task_dir "${task_id}")"
prompt_md="${task_dir}/prompt.md"
response_md="${task_dir}/response.md"
result_json="${task_dir}/result.json"

[[ -f "${task_dir}/task.json" ]] || { echo "missing task.json for ${task_id}" >&2; exit 1; }
eval "$("$(dorkpipe_orchestrate_helper_bin)" task-env "${task_dir}/task.json")"
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
estimated_output_tokens="0"
selected_model="$("$(dorkpipe_orchestrate_helper_bin)" task-model)"

append_dependency_context() {
  [[ "$(dorkpipe_orchestrate_bool "${DORKPIPE_ORCH_APPEND_DEPENDENCY_CONTEXT}")" == "true" ]] || return 0
  "$(dorkpipe_orchestrate_helper_bin)" append-dependency-context "${prompt_md}" "${DORKPIPE_ORCH_TASKS_DIR}" "${TASK_DEPENDS_ON_JSON:-[]}" "${DORKPIPE_ORCH_DEPENDENCY_CONTEXT_MAX_BYTES}" "${DORKPIPE_ORCH_DEPENDENCY_CONTEXT_TOTAL_MAX_BYTES}" "${DORKPIPE_ORCH_PREFER_PLANNER_CONTEXT}"
}

append_dependency_context
estimated_input_tokens="$(dorkpipe_orchestrate_estimate_tokens_for_file "${prompt_md}")"
estimated_total_tokens="${estimated_input_tokens}"

live_response_is_valid() {
  local path="${1:?response path}"
  [[ -s "${path}" ]] || return 1
  "$(dorkpipe_orchestrate_helper_bin)" validate-live-response "${path}" >/dev/null
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
  elif (( $(dorkpipe_orchestrate_read_usage_number "total_estimated_tokens") + estimated_input_tokens > DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS )) && [[ "$(dorkpipe_orchestrate_bool "${DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED}")" == "true" ]]; then
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
          if dorkpipe_orchestrate_run_container_worker codex "${prompt_md}" "${response_md}" "${selected_model}" && live_response_is_valid "${response_md}"; then
            used_live_model="true"
            summary="Live Codex worker output captured in response.md from the codex resolver container"
            confidence="0.72"
            issues_json='[]'
            next_actions_json='["merge this task with sibling worker outputs"]'
          else
            issues_json='["codex resolver container failed, host Codex auth was unavailable, or output was not a model response"]'
          fi
        elif command -v codex >/dev/null 2>&1; then
          codex_args=("exec" "--dangerously-bypass-approvals-and-sandbox")
          if [[ -n "${selected_model}" && "${selected_model}" != "cli" ]]; then
            codex_args+=("--model" "${selected_model}")
          fi
          codex_args+=("$(cat "${prompt_md}")")
          if codex "${codex_args[@]}" > "${response_md}" && live_response_is_valid "${response_md}"; then
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
          if dorkpipe_orchestrate_run_container_worker claude "${prompt_md}" "${response_md}" "${selected_model}" && live_response_is_valid "${response_md}"; then
            used_live_model="true"
            summary="Live Claude worker output captured in response.md from the claude resolver container"
            confidence="0.72"
            issues_json='[]'
            next_actions_json='["merge this task with sibling worker outputs"]'
          else
            issues_json='["claude resolver container failed, host Claude auth was unavailable, or output was not a model response"]'
          fi
        elif command -v claude >/dev/null 2>&1; then
          claude_args=()
          if [[ -n "${selected_model}" && "${selected_model}" != "cli" ]]; then
            claude_args+=("--model" "${selected_model}")
          fi
          claude_args+=("-p" "$(cat "${prompt_md}")")
          if claude "${claude_args[@]}" > "${response_md}" && live_response_is_valid "${response_md}"; then
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
          ollama_model="${selected_model}"
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
task_finished_at_ms="$(dorkpipe_orchestrate_now_ms)"
task_finished_at="$(dorkpipe_orchestrate_now_iso)"
duration_ms="$(( task_finished_at_ms - task_started_at_ms ))"

if [[ "${used_live_model}" == "true" ]] && dorkpipe_orchestrate_is_cloud_provider "${provider}"; then
  dorkpipe_orchestrate_record_cloud_usage "${provider}" "${estimated_input_tokens}" "${estimated_output_tokens}" "${duration_ms}"
fi

dorkpipe_orchestrate_record_training_metric "${task_id}" "${lane_id}" "${provider}" "${status}" "${confidence}" "${estimated_input_tokens}" "${estimated_output_tokens}" "${used_live_model}" "${budget_halt}" "${task_started_at}" "${task_finished_at}" "${duration_ms}"

export task_id status resolver_hint provider lane_id selected_model used_live_model budget_halt estimated_input_tokens estimated_output_tokens estimated_total_tokens task_started_at task_finished_at duration_ms summary confidence issues_json next_actions_json TASK_BASE_ID TASK_COMPARISON_JSON TASK_LANE_JSON TASK_CLAIMS_JSON TASK_CITATIONS_JSON
"$(dorkpipe_orchestrate_helper_bin)" write-task-result "${result_json}"

printf '[dorkpipe] task %s result ready at %s\n' "${task_id}" "${result_json}" >&2
