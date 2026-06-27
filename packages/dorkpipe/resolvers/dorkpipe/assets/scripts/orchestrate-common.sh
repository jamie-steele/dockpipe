#!/usr/bin/env bash
set -euo pipefail

dorkpipe_orchestrate_init() {
  eval "$(dockpipe sdk)"
  dockpipe_sdk init-script
  export ROOT="${ROOT:-${DOCKPIPE_WORKDIR:-$(pwd)}}"
  export DORKPIPE_ORCH_WORKFLOW="${DORKPIPE_ORCH_WORKFLOW:-docs.orchestrate}"
  export DORKPIPE_ORCH_ROOT="${DORKPIPE_ORCH_ROOT:-bin/.dockpipe/packages/dorkpipe/orchestrate/${DORKPIPE_ORCH_WORKFLOW}}"
  mkdir -p "${DORKPIPE_ORCH_ROOT}"
  export DORKPIPE_ORCH_REQUEST_JSON="${DORKPIPE_ORCH_REQUEST_JSON:-${DORKPIPE_ORCH_ROOT}/request.json}"
  export DORKPIPE_ORCH_PLAN_JSON="${DORKPIPE_ORCH_PLAN_JSON:-${DORKPIPE_ORCH_ROOT}/plan.json}"
  export DORKPIPE_ORCH_GRAPH_JSON="${DORKPIPE_ORCH_GRAPH_JSON:-${DORKPIPE_ORCH_ROOT}/task-graph.json}"
  export DORKPIPE_ORCH_SHARED_DIR="${DORKPIPE_ORCH_SHARED_DIR:-${DORKPIPE_ORCH_ROOT}/shared}"
  export DORKPIPE_ORCH_TASKS_DIR="${DORKPIPE_ORCH_TASKS_DIR:-${DORKPIPE_ORCH_ROOT}/tasks}"
  export DORKPIPE_ORCH_MERGE_DIR="${DORKPIPE_ORCH_MERGE_DIR:-${DORKPIPE_ORCH_ROOT}/merge}"
  export DORKPIPE_ORCH_VERIFY_DIR="${DORKPIPE_ORCH_VERIFY_DIR:-${DORKPIPE_ORCH_ROOT}/verify}"
  export DORKPIPE_ORCH_APPLY_DIR="${DORKPIPE_ORCH_APPLY_DIR:-${DORKPIPE_ORCH_ROOT}/apply}"
  export DORKPIPE_ORCH_LANES_DIR="${DORKPIPE_ORCH_LANES_DIR:-${DORKPIPE_ORCH_ROOT}/lanes}"
  export DORKPIPE_ORCH_TRAINING_DIR="${DORKPIPE_ORCH_TRAINING_DIR:-${DORKPIPE_ORCH_ROOT}/training}"
  export DORKPIPE_ORCH_APPROVAL_MD="${DORKPIPE_ORCH_APPROVAL_MD:-${DORKPIPE_ORCH_ROOT}/approval.md}"
  export DORKPIPE_ORCH_CLOUD_USAGE_JSON="${DORKPIPE_ORCH_CLOUD_USAGE_JSON:-${DORKPIPE_ORCH_ROOT}/cloud-usage.json}"
  export DORKPIPE_ORCH_HALT_JSON="${DORKPIPE_ORCH_HALT_JSON:-${DORKPIPE_ORCH_ROOT}/halt.json}"
  export DORKPIPE_ORCH_MODEL_CATALOG="${DORKPIPE_ORCH_MODEL_CATALOG:-${DOCKPIPE_ASSETS_DIR:-$(cd "${SCRIPT_DIR}/.." && pwd)}/model-lanes/catalog.yml}"
  export DORKPIPE_ORCH_BASELINE_POLICY="${DORKPIPE_ORCH_BASELINE_POLICY:-${DOCKPIPE_ASSETS_DIR:-$(cd "${SCRIPT_DIR}/.." && pwd)}/model-lanes/baseline-policy.yml}"
  export DORKPIPE_ORCH_LANE_PLAN_JSON="${DORKPIPE_ORCH_LANE_PLAN_JSON:-${DORKPIPE_ORCH_LANES_DIR}/plan.json}"
  export DORKPIPE_ORCH_TRAINING_METRICS_JSONL="${DORKPIPE_ORCH_TRAINING_METRICS_JSONL:-${DORKPIPE_ORCH_TRAINING_DIR}/metrics.jsonl}"
  export DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS="${DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS:-${ROOT}/bin/.dockpipe/packages/dorkpipe/training/metrics.jsonl}"
  export DORKPIPE_ORCH_TRAINING_MODE="${DORKPIPE_ORCH_TRAINING_MODE:-observe}"
  export DORKPIPE_ORCH_LIVE_MODELS="${DORKPIPE_ORCH_LIVE_MODELS:-true}"
  export DORKPIPE_ORCH_CLOUD_LANES="${DORKPIPE_ORCH_CLOUD_LANES:-false}"
  export DORKPIPE_ORCH_CONTAINERIZE_CLOUD="${DORKPIPE_ORCH_CONTAINERIZE_CLOUD:-true}"
  export DORKPIPE_ORCH_AUTH_MOUNT_MODE="${DORKPIPE_ORCH_AUTH_MOUNT_MODE:-rw}"
  export DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS="${DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS:-120000}"
  export DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS="${DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS:-40000}"
  export DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED="${DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED:-true}"
  mkdir -p "${DORKPIPE_ORCH_SHARED_DIR}" "${DORKPIPE_ORCH_TASKS_DIR}" "${DORKPIPE_ORCH_MERGE_DIR}" "${DORKPIPE_ORCH_VERIFY_DIR}" "${DORKPIPE_ORCH_APPLY_DIR}" "${DORKPIPE_ORCH_LANES_DIR}" "${DORKPIPE_ORCH_TRAINING_DIR}"
  if [[ ! -f "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" ]]; then
    cat > "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" <<EOF
{
  "max_total_cloud_tokens": ${DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS},
  "max_task_cloud_tokens": ${DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS},
  "stop_on_budget_exceeded": ${DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED},
  "total_estimated_input_tokens": 0,
  "total_estimated_output_tokens": 0,
  "total_estimated_tokens": 0,
  "cloud_task_count": 0,
  "budget_exceeded": false,
  "halted": false,
  "providers": {
    "codex": {"task_count": 0, "estimated_tokens": 0},
    "claude": {"task_count": 0, "estimated_tokens": 0}
  }
}
EOF
  fi
}

dorkpipe_orchestrate_task_dir() {
  local task_id="${1:?task id}"
  printf '%s\n' "${DORKPIPE_ORCH_TASKS_DIR}/${task_id}"
}

dorkpipe_orchestrate_resolve_provider() {
  local requested="${1:-auto}"
  if [[ -n "${task_dir:-}" && -f "${task_dir}/lane-selection.json" ]]; then
    local selected
    selected="$(sed -n 's/.*"provider": "\(.*\)".*/\1/p' "${task_dir}/lane-selection.json" | head -1)"
    if [[ -n "${selected}" ]]; then
      printf '%s\n' "${selected}"
      return 0
    fi
  fi
  if [[ -n "${DOCKPIPE_RESOLVER_CMD:-}" ]]; then
    printf '%s\n' "${DOCKPIPE_RESOLVER_CMD}"
    return 0
  fi
  if [[ "$requested" != "auto" ]]; then
    printf '%s\n' "$requested"
    return 0
  fi
  printf 'ollama\n'
}

dorkpipe_orchestrate_write_json() {
  local dest="${1:?dest}"
  shift || true
  mkdir -p "$(dirname "$dest")"
  cat > "$dest"
}

dorkpipe_orchestrate_json_escape() {
  printf '%s' "${1:-}" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

dorkpipe_orchestrate_bool() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|on|ON) printf 'true\n' ;;
    *) printf 'false\n' ;;
  esac
}

dorkpipe_orchestrate_is_cloud_provider() {
  case "${1:-}" in
    codex|claude) return 0 ;;
    *) return 1 ;;
  esac
}

dorkpipe_orchestrate_dockpipe_bin() {
  dockpipe_sdk require dockpipe-bin
}

dorkpipe_orchestrate_container_auth_dir() {
  local provider="${1:?provider}"
  case "${provider}" in
    codex)
      printf '%s\n' "${DORKPIPE_ORCH_CODEX_AUTH_DIR:-${CODEX_HOME:-${HOME:-}/.codex}}"
      ;;
    claude)
      printf '%s\n' "${DORKPIPE_ORCH_CLAUDE_AUTH_DIR:-${CLAUDE_HOME:-${HOME:-}/.claude}}"
      ;;
    *)
      return 1
      ;;
  esac
}

dorkpipe_orchestrate_container_auth_mount() {
  local provider="${1:?provider}"
  local host_dir container_dir mode
  host_dir="$(dorkpipe_orchestrate_container_auth_dir "${provider}")" || return 1
  [[ -n "${host_dir}" && -d "${host_dir}" ]] || return 1
  case "${provider}" in
    codex) container_dir="${DORKPIPE_ORCH_CODEX_CONTAINER_AUTH_DIR:-/home/node/.codex}" ;;
    claude) container_dir="${DORKPIPE_ORCH_CLAUDE_CONTAINER_AUTH_DIR:-/home/node/.claude}" ;;
    *) return 1 ;;
  esac
  mode="${DORKPIPE_ORCH_AUTH_MOUNT_MODE:-rw}"
  case "${mode}" in
    ro|rw) ;;
    *) mode="rw" ;;
  esac
  printf '%s:%s:%s\n' "${host_dir}" "${container_dir}" "${mode}"
}

dorkpipe_orchestrate_run_container_worker() {
  local provider="${1:?provider}"
  local prompt_path="${2:?prompt path}"
  local response_path="${3:?response path}"
  local dockpipe_bin auth_mount
  dockpipe_bin="$(dorkpipe_orchestrate_dockpipe_bin)" || return 1

  local args=(
    "--workdir" "${ROOT}"
    "--runtime" "dockerimage"
    "--resolver" "${provider}"
    "--no-data"
    "--env" "HOME=/home/node"
    "--env" "PATH=/usr/local/bin:/usr/bin:/bin:/usr/local/games:/usr/games"
  )
  if auth_mount="$(dorkpipe_orchestrate_container_auth_mount "${provider}" 2>/dev/null)"; then
    args+=("--mount" "${auth_mount}")
  fi

  case "${provider}" in
    codex)
      "${dockpipe_bin}" "${args[@]}" -- \
        codex exec --dangerously-bypass-approvals-and-sandbox "$(cat "${prompt_path}")" > "${response_path}"
      ;;
    claude)
      "${dockpipe_bin}" "${args[@]}" -- \
        claude --dangerously-skip-permissions -p "$(cat "${prompt_path}")" > "${response_path}"
      ;;
    *)
      return 1
      ;;
  esac
}

dorkpipe_orchestrate_estimate_tokens_for_file() {
  local path="${1:?path}"
  local chars="0"
  if [[ -f "${path}" ]]; then
    chars="$(wc -c < "${path}" | tr -d '[:space:]')"
  fi
  printf '%s\n' "$(( (chars + 3) / 4 ))"
}

dorkpipe_orchestrate_read_usage_number() {
  local key="${1:?key}"
  local value
  value="$(sed -n "s/.*\"${key}\": \\([0-9][0-9]*\\).*/\\1/p" "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" | head -1)"
  printf '%s\n' "${value:-0}"
}

dorkpipe_orchestrate_read_provider_usage_number() {
  local provider="${1:?provider}"
  local field="${2:?field}"
  local capture="1"
  if [[ "${field}" == "estimated_tokens" ]]; then
    capture="2"
  fi
  local value
  value="$(sed -n "s/.*\"${provider}\": {\"task_count\": \\([0-9][0-9]*\\), \"estimated_tokens\": \\([0-9][0-9]*\\)}.*/\\${capture}/p" "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" | head -1)"
  printf '%s\n' "${value:-0}"
}

dorkpipe_orchestrate_halt_run() {
  local provider="${1:-unknown}"
  local reason="${2:-budget exceeded}"
  cat > "${DORKPIPE_ORCH_HALT_JSON}" <<EOF
{
  "status": "halted",
  "provider": "$(dorkpipe_orchestrate_json_escape "${provider}")",
  "reason": "$(dorkpipe_orchestrate_json_escape "${reason}")"
}
EOF
  if [[ -f "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" ]]; then
    local total_input total_output total_tokens task_count exceeded codex_task_count codex_tokens claude_task_count claude_tokens
    total_input="$(dorkpipe_orchestrate_read_usage_number "total_estimated_input_tokens")"
    total_output="$(dorkpipe_orchestrate_read_usage_number "total_estimated_output_tokens")"
    total_tokens="$(dorkpipe_orchestrate_read_usage_number "total_estimated_tokens")"
    task_count="$(dorkpipe_orchestrate_read_usage_number "cloud_task_count")"
    exceeded="$(sed -n 's/.*"budget_exceeded": \(true\|false\).*/\1/p' "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" | head -1)"
    exceeded="${exceeded:-false}"
    codex_task_count="$(dorkpipe_orchestrate_read_provider_usage_number "codex" "task_count")"
    codex_tokens="$(dorkpipe_orchestrate_read_provider_usage_number "codex" "estimated_tokens")"
    claude_task_count="$(dorkpipe_orchestrate_read_provider_usage_number "claude" "task_count")"
    claude_tokens="$(dorkpipe_orchestrate_read_provider_usage_number "claude" "estimated_tokens")"
    cat > "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" <<EOF
{
  "max_total_cloud_tokens": ${DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS},
  "max_task_cloud_tokens": ${DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS},
  "stop_on_budget_exceeded": ${DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED},
  "total_estimated_input_tokens": ${total_input},
  "total_estimated_output_tokens": ${total_output},
  "total_estimated_tokens": ${total_tokens},
  "cloud_task_count": ${task_count},
  "budget_exceeded": ${exceeded},
  "halted": true,
  "providers": {
    "codex": {
      "task_count": ${codex_task_count},
      "estimated_tokens": ${codex_tokens}
    },
    "claude": {
      "task_count": ${claude_task_count},
      "estimated_tokens": ${claude_tokens}
    }
  }
}
EOF
  fi
}

dorkpipe_orchestrate_record_cloud_usage() {
  local provider="${1:?provider}"
  local input_tokens="${2:?input tokens}"
  local output_tokens="${3:?output tokens}"
  local total_tokens new_total_input new_total_output new_total_tokens new_task_count
  local provider_task_count provider_tokens budget_exceeded halted
  local codex_task_count codex_tokens claude_task_count claude_tokens
  total_tokens="$(( input_tokens + output_tokens ))"
  new_total_input="$(( $(dorkpipe_orchestrate_read_usage_number "total_estimated_input_tokens") + input_tokens ))"
  new_total_output="$(( $(dorkpipe_orchestrate_read_usage_number "total_estimated_output_tokens") + output_tokens ))"
  new_total_tokens="$(( $(dorkpipe_orchestrate_read_usage_number "total_estimated_tokens") + total_tokens ))"
  new_task_count="$(( $(dorkpipe_orchestrate_read_usage_number "cloud_task_count") + 1 ))"
  provider_task_count="$(( $(dorkpipe_orchestrate_read_provider_usage_number "${provider}" "task_count") + 1 ))"
  provider_tokens="$(( $(dorkpipe_orchestrate_read_provider_usage_number "${provider}" "estimated_tokens") + total_tokens ))"
  codex_task_count="$(dorkpipe_orchestrate_read_provider_usage_number "codex" "task_count")"
  codex_tokens="$(dorkpipe_orchestrate_read_provider_usage_number "codex" "estimated_tokens")"
  claude_task_count="$(dorkpipe_orchestrate_read_provider_usage_number "claude" "task_count")"
  claude_tokens="$(dorkpipe_orchestrate_read_provider_usage_number "claude" "estimated_tokens")"
  if [[ "${provider}" == "codex" ]]; then
    codex_task_count="${provider_task_count}"
    codex_tokens="${provider_tokens}"
  fi
  if [[ "${provider}" == "claude" ]]; then
    claude_task_count="${provider_task_count}"
    claude_tokens="${provider_tokens}"
  fi
  budget_exceeded="false"
  if (( new_total_tokens > DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS )); then
    budget_exceeded="true"
  fi
  halted="false"
  if [[ -f "${DORKPIPE_ORCH_HALT_JSON}" ]]; then
    halted="true"
  fi
  cat > "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" <<EOF
{
  "max_total_cloud_tokens": ${DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS},
  "max_task_cloud_tokens": ${DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS},
  "stop_on_budget_exceeded": ${DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED},
  "total_estimated_input_tokens": ${new_total_input},
  "total_estimated_output_tokens": ${new_total_output},
  "total_estimated_tokens": ${new_total_tokens},
  "cloud_task_count": ${new_task_count},
  "budget_exceeded": ${budget_exceeded},
  "halted": ${halted},
  "providers": {
    "codex": {
      "task_count": ${codex_task_count},
      "estimated_tokens": ${codex_tokens}
    },
    "claude": {
      "task_count": ${claude_task_count},
      "estimated_tokens": ${claude_tokens}
    }
  }
}
EOF
  if [[ "${budget_exceeded}" == "true" && "$(dorkpipe_orchestrate_bool "${DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED}")" == "true" ]]; then
    dorkpipe_orchestrate_halt_run "${provider}" "Estimated cloud token budget exceeded after ${provider} task (${new_total_tokens}/${DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS})."
  fi
}

dorkpipe_orchestrate_record_training_metric() {
  local task_id="${1:?task id}"
  local lane_id="${2:-unknown}"
  local provider="${3:-unknown}"
  local status="${4:-unknown}"
  local confidence="${5:-0}"
  local input_tokens="${6:-0}"
  local output_tokens="${7:-0}"
  local used_live_model="${8:-false}"
  local budget_halt="${9:-false}"
  mkdir -p "$(dirname "${DORKPIPE_ORCH_TRAINING_METRICS_JSONL}")"
  local metric
  metric='{"task_id":"'"$(dorkpipe_orchestrate_json_escape "${task_id}")"'","lane_id":"'"$(dorkpipe_orchestrate_json_escape "${lane_id}")"'","provider":"'"$(dorkpipe_orchestrate_json_escape "${provider}")"'","status":"'"$(dorkpipe_orchestrate_json_escape "${status}")"'","confidence":'"${confidence}"',"estimated_input_tokens":'"${input_tokens}"',"estimated_output_tokens":'"${output_tokens}"',"used_live_model":'"${used_live_model}"',"budget_halt":'"${budget_halt}"',"training_mode":"'"$(dorkpipe_orchestrate_json_escape "${DORKPIPE_ORCH_TRAINING_MODE}")"'"}'
  printf '%s\n' "${metric}" >> "${DORKPIPE_ORCH_TRAINING_METRICS_JSONL}"
  if [[ -n "${DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS:-}" && "${DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS}" != "${DORKPIPE_ORCH_TRAINING_METRICS_JSONL}" ]]; then
    mkdir -p "$(dirname "${DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS}")"
    printf '%s\n' "${metric}" >> "${DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS}"
  fi
}
