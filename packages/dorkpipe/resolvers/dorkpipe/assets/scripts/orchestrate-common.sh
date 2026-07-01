#!/usr/bin/env bash
set -euo pipefail

dorkpipe_orchestrate_init() {
  eval "$(dockpipe sdk)"
  dockpipe_sdk init-script
  export ROOT="${ROOT:-$(dockpipe_sdk get workdir)}"
  export DORKPIPE_ORCH_WORKFLOW="${DORKPIPE_ORCH_WORKFLOW:-${DOCKPIPE_WORKFLOW_NAME:-docs.orchestrate}}"
  default_orch_root="$(dockpipe scope artifacts orchestrate)"
  export DORKPIPE_ORCH_ROOT="${DORKPIPE_ORCH_ROOT:-${default_orch_root}}"
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
  export DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS="${DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS:-$(dockpipe scope --package dorkpipe training metrics.jsonl)}"
  export DORKPIPE_ORCH_TRAINING_MODE="${DORKPIPE_ORCH_TRAINING_MODE:-observe}"
  export DORKPIPE_ORCH_LIVE_MODELS="${DORKPIPE_ORCH_LIVE_MODELS:-true}"
  export DORKPIPE_ORCH_CLOUD_LANES="${DORKPIPE_ORCH_CLOUD_LANES:-false}"
  export DORKPIPE_ORCH_FORCE_PROVIDER="${DORKPIPE_ORCH_FORCE_PROVIDER:-${DORKPIPE_ORCH_TASK_PROVIDER:-}}"
  export DORKPIPE_ORCH_FORCE_PROVIDER_SCOPE="${DORKPIPE_ORCH_FORCE_PROVIDER_SCOPE:-auto}"
  export DORKPIPE_ORCH_COMPARE_PROVIDERS="${DORKPIPE_ORCH_COMPARE_PROVIDERS:-}"
  export DORKPIPE_ORCH_COMPARE_SCOPE="${DORKPIPE_ORCH_COMPARE_SCOPE:-auto}"
  export DORKPIPE_ORCH_COMPARE_ANIMATION="${DORKPIPE_ORCH_COMPARE_ANIMATION:-auto}"
  export DORKPIPE_ORCH_COMPARE_RENDERER="${DORKPIPE_ORCH_COMPARE_RENDERER:-clear}"
  export DORKPIPE_ORCH_COMPARE_WORKER_LOGS="${DORKPIPE_ORCH_COMPARE_WORKER_LOGS:-artifact}"
  export DORKPIPE_ORCH_STRICT_OUTPUT_CONTRACT="${DORKPIPE_ORCH_STRICT_OUTPUT_CONTRACT:-true}"
  export DORKPIPE_ORCH_INLINE_INPUT_CONTEXT="${DORKPIPE_ORCH_INLINE_INPUT_CONTEXT:-true}"
  export DORKPIPE_ORCH_INLINE_INPUT_MAX_BYTES="${DORKPIPE_ORCH_INLINE_INPUT_MAX_BYTES:-6000}"
  export DORKPIPE_ORCH_INLINE_INPUT_TOTAL_MAX_BYTES="${DORKPIPE_ORCH_INLINE_INPUT_TOTAL_MAX_BYTES:-18000}"
  export DORKPIPE_ORCH_LOCAL_INCLUDE_AGENTS_MD="${DORKPIPE_ORCH_LOCAL_INCLUDE_AGENTS_MD:-false}"
  export DORKPIPE_ORCH_APPEND_DEPENDENCY_CONTEXT="${DORKPIPE_ORCH_APPEND_DEPENDENCY_CONTEXT:-true}"
  export DORKPIPE_ORCH_PREFER_PLANNER_CONTEXT="${DORKPIPE_ORCH_PREFER_PLANNER_CONTEXT:-true}"
  export DORKPIPE_ORCH_DEPENDENCY_CONTEXT_MAX_BYTES="${DORKPIPE_ORCH_DEPENDENCY_CONTEXT_MAX_BYTES:-5000}"
  export DORKPIPE_ORCH_DEPENDENCY_CONTEXT_TOTAL_MAX_BYTES="${DORKPIPE_ORCH_DEPENDENCY_CONTEXT_TOTAL_MAX_BYTES:-12000}"
  export DORKPIPE_ORCH_FANOUT_PROVIDER="${DORKPIPE_ORCH_FANOUT_PROVIDER:-}"
  export DORKPIPE_ORCH_CONTAINERIZE_CLOUD="${DORKPIPE_ORCH_CONTAINERIZE_CLOUD:-true}"
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
  fi
}

dorkpipe_orchestrate_helper_bin() {
  if [[ -n "${DORKPIPE_ORCH_HELPER_BIN:-}" ]]; then
    printf '%s\n' "${DORKPIPE_ORCH_HELPER_BIN}"
    return 0
  fi
  local repo_root dockpipe_bin
  repo_root="${ROOT:-$(dockpipe_sdk get workdir)}"
  DORKPIPE_ORCH_HELPER_BIN="${DOCKPIPE_ASSETS_DIR:-}/tooling/bin/$(case "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" in Windows_NT:*|*:msys*:*|*:cygwin*:*|*:*:MINGW*) printf 'windows' ;; darwin*:*|*:darwin*:* ) printf 'darwin' ;; *) printf 'linux' ;; esac)/orchestrate-helper$(case "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" in Windows_NT:*|*:msys*:*|*:cygwin*:*|*:*:MINGW*) printf '.exe' ;; *) printf '' ;; esac)"
  if [[ ! -x "${DORKPIPE_ORCH_HELPER_BIN}" ]]; then
    DORKPIPE_ORCH_HELPER_BIN="$(dockpipe_sdk require tooling-bin orchestrate-helper || true)"
  fi
  if [[ -n "${DORKPIPE_ORCH_HELPER_BIN}" ]]; then
    export DORKPIPE_ORCH_HELPER_BIN
    printf '%s\n' "${DORKPIPE_ORCH_HELPER_BIN}"
    return 0
  fi
  dockpipe_bin="${DOCKPIPE_BIN:-}"
  if [[ -z "${dockpipe_bin}" ]]; then
    dockpipe_bin="$(dockpipe_sdk require dockpipe-bin || true)"
  fi
  if [[ -x "${dockpipe_bin:-}" ]]; then
    "${dockpipe_bin}" package build source --workdir "${repo_root}" --only dorkpipe
    DORKPIPE_ORCH_HELPER_BIN="$(dockpipe_sdk require tooling-bin orchestrate-helper || true)"
    if [[ -n "${DORKPIPE_ORCH_HELPER_BIN}" ]]; then
      export DORKPIPE_ORCH_HELPER_BIN
      printf '%s\n' "${DORKPIPE_ORCH_HELPER_BIN}"
      return 0
    fi
  fi
  echo "orchestrate-helper: compiled helper not found at ${repo_root}/bin/.dockpipe/tooling/bin/orchestrate-helper(.exe)" >&2
  echo "Run: ${dockpipe_bin:-dockpipe} package build source --workdir ${repo_root} --only dorkpipe" >&2
  return 1
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

dorkpipe_orchestrate_now_ms() {
  date +%s%3N
}

dorkpipe_orchestrate_now_iso() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
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

dorkpipe_orchestrate_cli_mount_host_path() {
  local path="${1:-}"
  if [[ "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" == Windows_NT:* || "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" == *:msys*:* || "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" == *:cygwin*:* || "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" == *:*:MINGW* ]]; then
    if [[ "${path}" =~ ^([A-Za-z]):[\\/] ]]; then
      local drive rest
      drive="$(printf '%s' "${BASH_REMATCH[1]}" | tr '[:upper:]' '[:lower:]')"
      rest="${path:2}"
      rest="${rest//\\//}"
      rest="${rest#/}"
      printf '/%s/%s\n' "${drive}" "${rest}"
      return 0
    fi
  fi
  printf '%s\n' "${path}"
}

dorkpipe_orchestrate_cli_mount_spec() {
  local spec="${1:-}"
  if [[ "${spec}" =~ ^([A-Za-z]:[\\/].*):(/.*)$ ]]; then
    printf '%s:%s\n' "$(dorkpipe_orchestrate_cli_mount_host_path "${BASH_REMATCH[1]}")" "${BASH_REMATCH[2]}"
    return 0
  fi
  printf '%s\n' "${spec}"
}

dorkpipe_orchestrate_container_auth_dir() {
  local provider="${1:?provider}"
  dockpipe scope resolver "${provider}" auth-dir
}

dorkpipe_orchestrate_container_auth_mount() {
  local provider="${1:?provider}"
  local host_dir container_dir mode
  host_dir="$(dorkpipe_orchestrate_container_auth_dir "${provider}")" || return 1
  [[ -n "${host_dir}" && -d "${host_dir}" ]] || return 1
  container_dir="$(dockpipe scope resolver "${provider}" container-auth-dir)" || return 1
  [[ -n "${container_dir}" ]] || return 1
  mode="$(dockpipe scope resolver "${provider}" auth-mount-mode)"
  case "${mode}" in
    ro|rw) ;;
    *) mode="rw" ;;
  esac
  host_dir="$(dorkpipe_orchestrate_cli_mount_host_path "${host_dir}")"
  printf '%s:%s:%s\n' "${host_dir}" "${container_dir}" "${mode}"
}

dorkpipe_orchestrate_container_auth_seed_mount() {
  local provider="${1:?provider}"
  local host_dir
  host_dir="$(dorkpipe_orchestrate_container_auth_dir "${provider}")" || return 1
  [[ -n "${host_dir}" && -d "${host_dir}" ]] || return 1
  host_dir="$(dorkpipe_orchestrate_cli_mount_host_path "${host_dir}")"
  printf '%s:/dockpipe-auth/%s:ro\n' "${host_dir}" "${provider}"
}

dorkpipe_orchestrate_container_extra_auth_mounts() {
  local provider="${1:?provider}"
  local mode host_file container_file
  mode="$(dockpipe scope resolver "${provider}" auth-mount-mode)"
  case "${mode}" in
    ro|rw) ;;
    *) mode="rw" ;;
  esac
  case "${provider}" in
    claude)
      host_file="$(dockpipe scope resolver "${provider}" config-file)"
      if [[ -n "${host_file}" && -f "${host_file}" ]]; then
        host_file="$(dorkpipe_orchestrate_cli_mount_host_path "${host_file}")"
        printf '%s:/dockpipe-auth/%s-config/.claude.json:ro\n' "${host_file}" "${provider}"
      fi
      ;;
  esac
}

dorkpipe_orchestrate_container_workflow_mounts() {
  local mount
  while IFS= read -r mount; do
    mount="${mount#"${mount%%[![:space:]]*}"}"
    mount="${mount%"${mount##*[![:space:]]}"}"
    [[ -n "${mount}" ]] || continue
    dorkpipe_orchestrate_cli_mount_spec "${mount}"
  done <<< "${DOCKPIPE_CONTAINER_MOUNTS:-}"
}

dorkpipe_orchestrate_worker_cwd() {
  local cwd="${DORKPIPE_ORCH_WORKER_CWD:-${DORKPIPE_ORCH_TARGET_GUEST_PATH:-}}"
  cwd="${cwd#"${cwd%%[![:space:]]*}"}"
  cwd="${cwd%"${cwd##*[![:space:]]}"}"
  # Git Bash converts env values like /UniteHere to its install root when launching
  # Windows child processes. Convert that shape back to the intended guest path.
  if [[ "${cwd}" =~ ^[A-Za-z]:[\\/](Program[[:space:]]Files|Program[[:space:]]Files[[:space:]]\(x86\))[\\/]Git[\\/](.+)$ ]]; then
    cwd="/${BASH_REMATCH[2]//\\//}"
  elif [[ "${cwd}" =~ ^/[A-Za-z]/(Program[[:space:]]Files|Program[[:space:]]Files[[:space:]]\(x86\))/Git/(.+)$ ]]; then
    cwd="/${BASH_REMATCH[2]}"
  fi
  if [[ -z "${cwd}" ]]; then
    printf '/work\n'
    return 0
  fi
  case "${cwd}" in
    /*) printf '%s\n' "${cwd}" ;;
    *) printf '/work/%s\n' "${cwd}" ;;
  esac
}

dorkpipe_orchestrate_container_auth_envs() {
  local provider="${1:?provider}"
  case "${provider}" in
    codex)
      printf '%s\n' \
        "CODEX_HOME=/home/node/.codex" \
        "DOCKPIPE_POLICY_NETWORK_MODE=internet"
      if [[ -n "${OPENAI_API_KEY:-}" ]]; then
        printf '%s\n' "OPENAI_API_KEY=${OPENAI_API_KEY}"
      fi
      if [[ -n "${CODEX_API_KEY:-}" ]]; then
        printf '%s\n' "CODEX_API_KEY=${CODEX_API_KEY}"
      fi
      ;;
    claude)
      printf '%s\n' \
        "CLAUDE_HOME=/home/node/.claude" \
        "DOCKPIPE_POLICY_NETWORK_MODE=internet"
      if [[ -n "${ANTHROPIC_API_KEY:-}" ]]; then
        printf '%s\n' "ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}"
      fi
      if [[ -n "${CLAUDE_API_KEY:-}" ]]; then
        printf '%s\n' "CLAUDE_API_KEY=${CLAUDE_API_KEY}"
      fi
      ;;
  esac
}

dorkpipe_orchestrate_auth_state_hint() {
  local provider="${1:?provider}"
  local host_dir host_file
  case "${provider}" in
    codex)
      if [[ -n "${OPENAI_API_KEY:-}" || -n "${CODEX_API_KEY:-}" ]]; then
        return 0
      fi
      host_dir="$(dorkpipe_orchestrate_container_auth_dir "${provider}" 2>/dev/null || true)"
      [[ -n "${host_dir}" && -s "${host_dir}/auth.json" ]] && return 0
      echo "codex auth unavailable: set OPENAI_API_KEY/CODEX_API_KEY or run codex login so ${host_dir:-<codex auth dir>}/auth.json exists" >&2
      return 1
      ;;
    claude)
      if [[ -n "${ANTHROPIC_API_KEY:-}" || -n "${CLAUDE_API_KEY:-}" ]]; then
        return 0
      fi
      host_dir="$(dorkpipe_orchestrate_container_auth_dir "${provider}" 2>/dev/null || true)"
      host_file="$(dockpipe scope resolver "${provider}" config-file 2>/dev/null || true)"
      [[ -n "${host_dir}" && -s "${host_dir}/.credentials.json" ]] && return 0
      [[ -n "${host_file}" && -s "${host_file}" ]] && return 0
      echo "claude auth unavailable: set ANTHROPIC_API_KEY/CLAUDE_API_KEY or run claude login so ${host_dir:-<claude auth dir>} contains credentials" >&2
      return 1
      ;;
  esac
  return 0
}

dorkpipe_orchestrate_run_container_worker() {
  local provider="${1:?provider}"
  local prompt_path="${2:?prompt path}"
  local response_path="${3:?response path}"
  local selected_model="${4:-}"
  local dockpipe_bin auth_mount raw_response_path worker_cwd
  dockpipe_bin="$(dorkpipe_orchestrate_dockpipe_bin)" || return 1
  raw_response_path="${response_path}.raw"
  worker_cwd="$(dorkpipe_orchestrate_worker_cwd)"
  dorkpipe_orchestrate_auth_state_hint "${provider}" || return 1

  local args=(
    "--workdir" "${ROOT}"
    "--runtime" "dockerimage"
    "--resolver" "${provider}"
    "--no-data"
    "--env" "HOME=/home/node"
    "--env" "PATH=/usr/local/bin:/usr/bin:/bin:/usr/local/games:/usr/games"
  )
  while IFS= read -r auth_env; do
    [[ -n "${auth_env}" ]] || continue
    args+=("--env" "${auth_env}")
  done < <(dorkpipe_orchestrate_container_auth_envs "${provider}")
  if auth_mount="$(dorkpipe_orchestrate_container_auth_seed_mount "${provider}" 2>/dev/null)"; then
    args+=("--mount" "${auth_mount}")
  fi
  while IFS= read -r auth_mount; do
    [[ -n "${auth_mount}" ]] || continue
    args+=("--mount" "${auth_mount}")
  done < <(dorkpipe_orchestrate_container_extra_auth_mounts "${provider}" 2>/dev/null || true)
  while IFS= read -r workflow_mount; do
    [[ -n "${workflow_mount}" ]] || continue
    args+=("--mount" "${workflow_mount}")
  done < <(dorkpipe_orchestrate_container_workflow_mounts 2>/dev/null || true)

  case "${provider}" in
    codex)
      MSYS2_ARG_CONV_EXCL='*' "${dockpipe_bin}" "${args[@]}" -- \
        bash -lc '
          set -euo pipefail
          export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin${PATH:+:$PATH}"
          if [[ -n "${3:-}" ]]; then
            cd "$3"
          fi
          mkdir -p /home/node/.codex
          if [[ -d /dockpipe-auth/codex ]]; then
            for item in auth.json config.toml version.json installation_id .codex-global-state.json AGENTS.md; do
              if [[ -e "/dockpipe-auth/codex/${item}" ]]; then
                cp -a "/dockpipe-auth/codex/${item}" /home/node/.codex/ 2>/dev/null || true
              fi
            done
            chmod -R u+rwX /home/node/.codex 2>/dev/null || true
          fi
          if [[ -n "${2:-}" && "${2:-}" != "cli" ]]; then
            codex exec --dangerously-bypass-approvals-and-sandbox --model "$2" "$1"
          else
            codex exec --dangerously-bypass-approvals-and-sandbox "$1"
          fi
        ' _ "$(cat "${prompt_path}")" "${selected_model}" "${worker_cwd}" > "${raw_response_path}"
      ;;
    claude)
      MSYS2_ARG_CONV_EXCL='*' "${dockpipe_bin}" "${args[@]}" -- \
        bash -lc '
          set -euo pipefail
          export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin${PATH:+:$PATH}"
          if [[ -n "${3:-}" ]]; then
            cd "$3"
          fi
          mkdir -p /home/node/.claude
          if [[ -d /dockpipe-auth/claude ]]; then
            for item in .credentials.json settings.json mcp-needs-auth-cache.json .last-cleanup; do
              if [[ -e "/dockpipe-auth/claude/${item}" ]]; then
                cp -a "/dockpipe-auth/claude/${item}" /home/node/.claude/ 2>/dev/null || true
              fi
            done
            if [[ -d /dockpipe-auth/claude/backups ]]; then
              mkdir -p /home/node/.claude/backups
              find /dockpipe-auth/claude/backups -maxdepth 1 -type f -name ".claude.json.backup.*" -exec cp -a {} /home/node/.claude/backups/ \; 2>/dev/null || true
            fi
            chmod -R u+rwX /home/node/.claude 2>/dev/null || true
          fi
          if [[ -f /dockpipe-auth/claude-config/.claude.json && ! -f /home/node/.claude.json ]]; then
            cp /dockpipe-auth/claude-config/.claude.json /home/node/.claude.json 2>/dev/null || true
            chmod u+rw /home/node/.claude.json 2>/dev/null || true
          fi
          if [[ ! -f /home/node/.claude.json && -d /home/node/.claude/backups ]]; then
            latest="$(find /home/node/.claude/backups -maxdepth 1 -type f -name ".claude.json.backup.*" -printf "%T@ %p\n" 2>/dev/null | sort -nr | head -1 | cut -d" " -f2-)"
            if [[ -n "${latest:-}" ]]; then
              cp "${latest}" /home/node/.claude.json
            fi
          fi
          if [[ -n "${2:-}" && "${2:-}" != "cli" ]]; then
            claude --dangerously-skip-permissions --model "$2" -p "$1"
          else
            claude --dangerously-skip-permissions -p "$1"
          fi
        ' _ "$(cat "${prompt_path}")" "${selected_model}" "${worker_cwd}" > "${raw_response_path}"
      ;;
    *)
      return 1
      ;;
  esac
  sed '/^sha256:[0-9a-f]\{64\}$/d' "${raw_response_path}" > "${response_path}"
  rm -f "${raw_response_path}"
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
  "$(dorkpipe_orchestrate_helper_bin)" usage-number "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" "${key}"
}

dorkpipe_orchestrate_read_provider_usage_number() {
  local provider="${1:?provider}"
  local field="${2:?field}"
  "$(dorkpipe_orchestrate_helper_bin)" provider-usage-number "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" "${provider}" "${field}"
}

dorkpipe_orchestrate_with_cloud_usage_lock() {
  local lock_dir="${DORKPIPE_ORCH_CLOUD_USAGE_JSON}.lock"
  local attempts=0
  local status=0
  until mkdir "${lock_dir}" 2>/dev/null; do
    attempts="$((attempts + 1))"
    if (( attempts > 500 )); then
      echo "timed out waiting for cloud usage lock: ${lock_dir}" >&2
      return 1
    fi
    sleep 0.02
  done
  "$@" || status="$?"
  rmdir "${lock_dir}" 2>/dev/null || true
  return "${status}"
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
    local total_input total_output total_tokens total_duration task_count exceeded codex_task_count codex_tokens codex_duration claude_task_count claude_tokens claude_duration
    total_input="$(dorkpipe_orchestrate_read_usage_number "total_estimated_input_tokens")"
    total_output="$(dorkpipe_orchestrate_read_usage_number "total_estimated_output_tokens")"
    total_tokens="$(dorkpipe_orchestrate_read_usage_number "total_estimated_tokens")"
    total_duration="$(dorkpipe_orchestrate_read_usage_number "total_duration_ms")"
    task_count="$(dorkpipe_orchestrate_read_usage_number "cloud_task_count")"
    exceeded="$(sed -n 's/.*"budget_exceeded": \(true\|false\).*/\1/p' "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" | head -1)"
    exceeded="${exceeded:-false}"
    codex_task_count="$(dorkpipe_orchestrate_read_provider_usage_number "codex" "task_count")"
    codex_tokens="$(dorkpipe_orchestrate_read_provider_usage_number "codex" "estimated_tokens")"
    codex_duration="$(dorkpipe_orchestrate_read_provider_usage_number "codex" "duration_ms")"
    claude_task_count="$(dorkpipe_orchestrate_read_provider_usage_number "claude" "task_count")"
    claude_tokens="$(dorkpipe_orchestrate_read_provider_usage_number "claude" "estimated_tokens")"
    claude_duration="$(dorkpipe_orchestrate_read_provider_usage_number "claude" "duration_ms")"
    cat > "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" <<EOF
{
  "max_total_cloud_tokens": ${DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS},
  "max_task_cloud_tokens": ${DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS},
  "stop_on_budget_exceeded": ${DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED},
  "total_estimated_input_tokens": ${total_input},
  "total_estimated_output_tokens": ${total_output},
  "total_estimated_tokens": ${total_tokens},
  "total_duration_ms": ${total_duration},
  "cloud_task_count": ${task_count},
  "budget_exceeded": ${exceeded},
  "halted": true,
  "providers": {
    "codex": {
      "task_count": ${codex_task_count},
      "estimated_tokens": ${codex_tokens},
      "duration_ms": ${codex_duration}
    },
    "claude": {
      "task_count": ${claude_task_count},
      "estimated_tokens": ${claude_tokens},
      "duration_ms": ${claude_duration}
    }
  }
}
EOF
  fi
}

dorkpipe_orchestrate_record_cloud_usage_unlocked() {
  local provider="${1:?provider}"
  local input_tokens="${2:?input tokens}"
  local output_tokens="${3:?output tokens}"
  local duration_ms="${4:-0}"
  local total_tokens new_total_input new_total_output new_total_tokens new_task_count
  local new_total_duration provider_task_count provider_tokens provider_duration budget_exceeded halted
  local codex_task_count codex_tokens codex_duration claude_task_count claude_tokens claude_duration
  total_tokens="$(( input_tokens + output_tokens ))"
  new_total_input="$(( $(dorkpipe_orchestrate_read_usage_number "total_estimated_input_tokens") + input_tokens ))"
  new_total_output="$(( $(dorkpipe_orchestrate_read_usage_number "total_estimated_output_tokens") + output_tokens ))"
  new_total_tokens="$(( $(dorkpipe_orchestrate_read_usage_number "total_estimated_tokens") + total_tokens ))"
  new_total_duration="$(( $(dorkpipe_orchestrate_read_usage_number "total_duration_ms") + duration_ms ))"
  new_task_count="$(( $(dorkpipe_orchestrate_read_usage_number "cloud_task_count") + 1 ))"
  provider_task_count="$(( $(dorkpipe_orchestrate_read_provider_usage_number "${provider}" "task_count") + 1 ))"
  provider_tokens="$(( $(dorkpipe_orchestrate_read_provider_usage_number "${provider}" "estimated_tokens") + total_tokens ))"
  provider_duration="$(( $(dorkpipe_orchestrate_read_provider_usage_number "${provider}" "duration_ms") + duration_ms ))"
  codex_task_count="$(dorkpipe_orchestrate_read_provider_usage_number "codex" "task_count")"
  codex_tokens="$(dorkpipe_orchestrate_read_provider_usage_number "codex" "estimated_tokens")"
  codex_duration="$(dorkpipe_orchestrate_read_provider_usage_number "codex" "duration_ms")"
  claude_task_count="$(dorkpipe_orchestrate_read_provider_usage_number "claude" "task_count")"
  claude_tokens="$(dorkpipe_orchestrate_read_provider_usage_number "claude" "estimated_tokens")"
  claude_duration="$(dorkpipe_orchestrate_read_provider_usage_number "claude" "duration_ms")"
  if [[ "${provider}" == "codex" ]]; then
    codex_task_count="${provider_task_count}"
    codex_tokens="${provider_tokens}"
    codex_duration="${provider_duration}"
  fi
  if [[ "${provider}" == "claude" ]]; then
    claude_task_count="${provider_task_count}"
    claude_tokens="${provider_tokens}"
    claude_duration="${provider_duration}"
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
  "total_duration_ms": ${new_total_duration},
  "cloud_task_count": ${new_task_count},
  "budget_exceeded": ${budget_exceeded},
  "halted": ${halted},
  "providers": {
    "codex": {
      "task_count": ${codex_task_count},
      "estimated_tokens": ${codex_tokens},
      "duration_ms": ${codex_duration}
    },
    "claude": {
      "task_count": ${claude_task_count},
      "estimated_tokens": ${claude_tokens},
      "duration_ms": ${claude_duration}
    }
  }
}
EOF
  if [[ "${budget_exceeded}" == "true" && "$(dorkpipe_orchestrate_bool "${DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED}")" == "true" ]]; then
    dorkpipe_orchestrate_halt_run "${provider}" "Estimated cloud token budget exceeded after ${provider} task (${new_total_tokens}/${DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS})."
  fi
}

dorkpipe_orchestrate_record_cloud_usage() {
  dorkpipe_orchestrate_with_cloud_usage_lock dorkpipe_orchestrate_record_cloud_usage_unlocked "$@"
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
  local started_at="${10:-}"
  local finished_at="${11:-}"
  local duration_ms="${12:-0}"
  mkdir -p "$(dirname "${DORKPIPE_ORCH_TRAINING_METRICS_JSONL}")"
  local metric
  metric='{"task_id":"'"$(dorkpipe_orchestrate_json_escape "${task_id}")"'","lane_id":"'"$(dorkpipe_orchestrate_json_escape "${lane_id}")"'","provider":"'"$(dorkpipe_orchestrate_json_escape "${provider}")"'","status":"'"$(dorkpipe_orchestrate_json_escape "${status}")"'","confidence":'"${confidence}"',"estimated_input_tokens":'"${input_tokens}"',"estimated_output_tokens":'"${output_tokens}"',"estimated_total_tokens":'"$(( input_tokens + output_tokens ))"',"started_at":"'"$(dorkpipe_orchestrate_json_escape "${started_at}")"'","finished_at":"'"$(dorkpipe_orchestrate_json_escape "${finished_at}")"'","duration_ms":'"${duration_ms}"',"used_live_model":'"${used_live_model}"',"budget_halt":'"${budget_halt}"',"training_mode":"'"$(dorkpipe_orchestrate_json_escape "${DORKPIPE_ORCH_TRAINING_MODE}")"'"}'
  printf '%s\n' "${metric}" >> "${DORKPIPE_ORCH_TRAINING_METRICS_JSONL}"
  if [[ -n "${DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS:-}" && "${DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS}" != "${DORKPIPE_ORCH_TRAINING_METRICS_JSONL}" ]]; then
    mkdir -p "$(dirname "${DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS}")"
    printf '%s\n' "${metric}" >> "${DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS}"
  fi
}
