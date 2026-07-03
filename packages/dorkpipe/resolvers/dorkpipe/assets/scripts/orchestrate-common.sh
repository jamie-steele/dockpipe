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
  export DORKPIPE_ORCH_WORK_MODE="${DORKPIPE_ORCH_WORK_MODE:-artifact}"
  export DORKPIPE_ORCH_EDIT_ISOLATION="${DORKPIPE_ORCH_EDIT_ISOLATION:-serialized}"
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
  export DORKPIPE_ORCH_CONTAINER_SKILLS="${DORKPIPE_ORCH_CONTAINER_SKILLS:-auto}"
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
  local repo_root package_root source_repo_root dockpipe_bin dockpipe_dir dockpipe_repo_root candidate repo_candidate packaged_candidate helper_sources_stale can_source_build
  repo_root="${ROOT:-$(dockpipe_sdk get workdir)}"
  if [[ -n "${DOCKPIPE_ASSETS_DIR:-}" ]]; then
    package_root="$(cd "${DOCKPIPE_ASSETS_DIR}/../../.." 2>/dev/null && pwd || true)"
  fi
  source_repo_root=""
  if [[ -n "${package_root:-}" ]]; then
    source_repo_root="$(cd "${package_root}/../.." 2>/dev/null && pwd || true)"
  fi
  repo_candidate="${repo_root}/bin/.dockpipe/tooling/bin/orchestrate-helper$(case "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" in Windows_NT:*|*:msys*:*|*:cygwin*:*|*:*:MINGW*) printf '.exe' ;; *) printf '' ;; esac)"
  packaged_candidate="${DOCKPIPE_ASSETS_DIR:-}/tooling/bin/$(case "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" in Windows_NT:*|*:msys*:*|*:cygwin*:*|*:*:MINGW*) printf 'windows' ;; darwin*:*|*:darwin*:* ) printf 'darwin' ;; *) printf 'linux' ;; esac)/orchestrate-helper$(case "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" in Windows_NT:*|*:msys*:*|*:cygwin*:*|*:*:MINGW*) printf '.exe' ;; *) printf '' ;; esac)"
  helper_sources_stale="0"
  can_source_build="0"
  if [[ -n "${source_repo_root:-}" && "${repo_root}" == "${source_repo_root}" ]] && [[ -d "${package_root}/lib/cmd/orchestrate-helper" ]] && [[ -d "${package_root}/lib/orchestrationhelper" ]]; then
    can_source_build="1"
  fi
  if [[ -x "${repo_candidate}" ]]; then
    if [[ "${can_source_build}" == "1" ]]; then
      if ! find "${package_root}/lib/cmd/orchestrate-helper" "${package_root}/lib/orchestrationhelper" \
        -type f \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) -newer "${repo_candidate}" -print -quit 2>/dev/null | grep -q .; then
        DORKPIPE_ORCH_HELPER_BIN="${repo_candidate}"
        export DORKPIPE_ORCH_HELPER_BIN
        printf '%s\n' "${DORKPIPE_ORCH_HELPER_BIN}"
        return 0
      fi
      helper_sources_stale="1"
    fi
  fi
  dockpipe_bin="${DOCKPIPE_BIN:-}"
  if [[ -z "${dockpipe_bin}" ]]; then
    dockpipe_bin="$(dockpipe_sdk require dockpipe-bin || true)"
  fi
  if [[ -x "${dockpipe_bin:-}" ]] && [[ "${helper_sources_stale}" != "1" ]] && [[ ! -x "${repo_candidate}" ]]; then
    dockpipe_dir="$(cd "$(dirname "${dockpipe_bin}")" && pwd)"
    dockpipe_repo_root="$(cd "${dockpipe_dir}/../.." 2>/dev/null && pwd || true)"
    if [[ "${dockpipe_repo_root}" != "/" && "${dockpipe_repo_root}" != "//" && -d "${dockpipe_repo_root}/packages/dorkpipe/lib/orchestrationhelper" ]]; then
      for candidate in \
        "${dockpipe_repo_root}/bin/.dockpipe/tooling/bin/orchestrate-helper" \
        "${dockpipe_repo_root}/bin/.dockpipe/tooling/bin/orchestrate-helper.exe"
      do
        if [[ -x "${candidate}" ]]; then
          DORKPIPE_ORCH_HELPER_BIN="${candidate}"
          export DORKPIPE_ORCH_HELPER_BIN
          printf '%s\n' "${DORKPIPE_ORCH_HELPER_BIN}"
          return 0
        fi
      done
    fi
  fi
  DORKPIPE_ORCH_HELPER_BIN="$(dockpipe_sdk require tooling-bin orchestrate-helper 2>/dev/null || true)"
  if [[ -n "${DORKPIPE_ORCH_HELPER_BIN}" && ( "${can_source_build}" != "1" || "${helper_sources_stale}" != "1" ) ]]; then
    export DORKPIPE_ORCH_HELPER_BIN
    printf '%s\n' "${DORKPIPE_ORCH_HELPER_BIN}"
    return 0
  fi
  if [[ "${can_source_build}" == "1" ]] && [[ -x "${dockpipe_bin:-}" ]] && { [[ "${helper_sources_stale}" == "1" ]] || [[ ! -x "${repo_candidate}" ]]; }; then
    "${dockpipe_bin}" package build source --workdir "${repo_root}" --only dorkpipe
    for candidate in \
      "${repo_root}/bin/.dockpipe/tooling/bin/orchestrate-helper" \
      "${repo_root}/bin/.dockpipe/tooling/bin/orchestrate-helper.exe"
    do
      if [[ -x "${candidate}" ]]; then
        DORKPIPE_ORCH_HELPER_BIN="${candidate}"
        export DORKPIPE_ORCH_HELPER_BIN
        printf '%s\n' "${DORKPIPE_ORCH_HELPER_BIN}"
        return 0
      fi
    done
    DORKPIPE_ORCH_HELPER_BIN="$(dockpipe_sdk require tooling-bin orchestrate-helper || true)"
    if [[ -n "${DORKPIPE_ORCH_HELPER_BIN}" ]]; then
      export DORKPIPE_ORCH_HELPER_BIN
      printf '%s\n' "${DORKPIPE_ORCH_HELPER_BIN}"
      return 0
    fi
  fi
  if [[ -x "${packaged_candidate}" ]]; then
    DORKPIPE_ORCH_HELPER_BIN="${packaged_candidate}"
    export DORKPIPE_ORCH_HELPER_BIN
    printf '%s\n' "${DORKPIPE_ORCH_HELPER_BIN}"
    return 0
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

dorkpipe_orchestrate_log_mode() {
  local mode
  mode="$(printf '%s' "${DORKPIPE_ORCH_LOG_MODE:-${DORKPIPE_LOG_MODE:-default}}" | tr '[:upper:]' '[:lower:]')"
  case "${mode}" in
    verbose|default|minimal|none) printf '%s\n' "${mode}" ;;
    quiet|silent|off) printf 'none\n' ;;
    "") printf 'default\n' ;;
    *)
      echo "[dorkpipe] unknown DORKPIPE_ORCH_LOG_MODE=${mode}; expected verbose, default, minimal, or none" >&2
      printf 'default\n'
      ;;
  esac
}

dorkpipe_orchestrate_log_is_verbose() {
  [[ "$(dorkpipe_orchestrate_log_mode)" == "verbose" ]]
}

dorkpipe_orchestrate_log_is_none() {
  [[ "$(dorkpipe_orchestrate_log_mode)" == "none" ]]
}

dorkpipe_orchestrate_log_supports_color() {
  [[ -t 2 && -z "${NO_COLOR:-}" ]]
}

dorkpipe_orchestrate_log_icon() {
  local kind="${1:-info}"
  if dorkpipe_orchestrate_log_supports_color; then
    case "${kind}" in
      ok) printf '\033[32m✓\033[0m' ;;
      fail) printf '\033[31m✗\033[0m' ;;
      *) printf '•' ;;
    esac
    return 0
  fi
  case "${kind}" in
    ok) printf '[ok]' ;;
    fail) printf '[fail]' ;;
    *) printf '[..]' ;;
  esac
}

dorkpipe_orchestrate_log_tail() {
  local log_path="${1:?log path}"
  local lines="${2:-40}"
  [[ -s "${log_path}" ]] || return 0
  echo "[dorkpipe] last ${lines} log lines from ${log_path}:" >&2
  tail -n "${lines}" "${log_path}" >&2 || true
}

dorkpipe_orchestrate_run_logged() {
  local label="${1:?label}"
  local log_path="${2:?log path}"
  shift 2
  local mode pid rc frame_index frame frames
  mode="$(dorkpipe_orchestrate_log_mode)"
  if [[ "${mode}" == "verbose" ]]; then
    "$@"
    return $?
  fi

  mkdir -p "$(dirname "${log_path}")"
  : > "${log_path}"
  if [[ "${mode}" != "none" ]]; then
    printf '[dorkpipe] %s ... ' "${label}" >&2
  fi
  "$@" > "${log_path}" 2>&1 &
  pid=$!
  frame_index=0
  frames='|/-\'
  while kill -0 "${pid}" 2>/dev/null; do
    if [[ "${mode}" != "none" && -t 2 ]]; then
      frame="${frames:${frame_index}:1}"
      printf '\r[dorkpipe] %s ... %s' "${label}" "${frame}" >&2
      frame_index=$(( (frame_index + 1) % 4 ))
    fi
    sleep 0.2
  done
  set +e
  wait "${pid}"
  rc=$?
  set -e
  if [[ "${mode}" != "none" ]]; then
    if [[ "${rc}" -eq 0 ]]; then
      printf '\r[dorkpipe] %s %s\n' "$(dorkpipe_orchestrate_log_icon ok)" "${label}" >&2
    else
      printf '\r[dorkpipe] %s %s\n' "$(dorkpipe_orchestrate_log_icon fail)" "${label}" >&2
      dorkpipe_orchestrate_log_tail "${log_path}" "${DORKPIPE_ORCH_LOG_FAILURE_LINES:-40}"
    fi
  fi
  return "${rc}"
}

dorkpipe_orchestrate_now_ms() {
  date +%s%3N
}

dorkpipe_orchestrate_now_iso() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

dorkpipe_orchestrate_result_escape() {
  printf '%s' "${1:-}" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

dorkpipe_orchestrate_operation_duration_ms() {
  local started_ms="${1:-0}"
  local finished_ms
  finished_ms="$(dorkpipe_orchestrate_now_ms)"
  printf '%s\n' "$(( finished_ms - started_ms ))"
}

dorkpipe_orchestrate_operation_emit() {
  local unit="${1:?unit}"
  local status="${2:?status}"
  local duration_ms="${3:-}"
  shift 3 || true
  local dockpipe_bin
  dockpipe_bin="$(dorkpipe_orchestrate_dockpipe_bin 2>/dev/null || true)"
  local args=(
    "result"
    "--unit" "${unit}"
    "--status" "${status}"
  )
  if [[ -n "${duration_ms}" && "${status}" != "start" ]]; then
    args+=("--duration-ms" "${duration_ms}")
  fi
  local field key value
  for field in "$@"; do
    [[ -n "${field}" ]] || continue
    if [[ "${field}" == *=* ]]; then
      key="${field%%=*}"
      value="${field#*=}"
      value="${value%\"}"
      value="${value#\"}"
      case "${key}" in
        error)
          args+=("--error" "${value}")
          ;;
        status)
          args+=("--id" "result_status=${value}")
          ;;
        *)
          args+=("--id" "${key}=${value}")
          ;;
      esac
    fi
  done
  if [[ -n "${dockpipe_bin}" ]]; then
    if "${dockpipe_bin}" "${args[@]}"; then
      return 0
    fi
    echo "[dorkpipe] warning: dockpipe result adapter failed; falling back to shell operation-result rendering" >&2
  fi
  printf '[dorkpipe] unit=%s status=%s' "${unit}" "${status}" >&2
  if [[ -n "${duration_ms}" && "${status}" != "start" ]]; then
    printf ' duration_ms=%s' "${duration_ms}" >&2
  fi
  for field in "$@"; do
    [[ -n "${field}" ]] || continue
    printf ' %s' "${field}" >&2
  done
  printf '\n' >&2
}

dorkpipe_orchestrate_operation_fail() {
  local unit="${1:?unit}"
  local started_ms="${2:?started ms}"
  local error_message="${3:-failed}"
  shift 3 || true
  local duration_ms
  duration_ms="$(dorkpipe_orchestrate_operation_duration_ms "${started_ms}")"
  dorkpipe_orchestrate_operation_emit "${unit}" "fail" "${duration_ms}" "$@" "error=\"$(dorkpipe_orchestrate_result_escape "${error_message}")\""
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
  # Worker launches set MSYS2_ARG_CONV_EXCL='*' so guest paths such as
  # /home/node and /UniteHere are not rewritten by Git Bash. Because that also
  # disables /c/... -> C:\... conversion, keep Windows host paths in native form
  # for the Windows dockpipe.exe.
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

dorkpipe_orchestrate_host_auth_candidates() {
  local provider="${1:?provider}"
  local host_dir
  host_dir="$(dorkpipe_orchestrate_container_auth_dir "${provider}" 2>/dev/null || true)"
  [[ -n "${host_dir}" ]] && printf '%s\n' "${host_dir}"
  set +e
  case "${provider}" in
    codex)
      [[ -n "${HOME:-}" ]] && printf '%s\n' "${HOME}/.codex"
      [[ -n "${USERPROFILE:-}" ]] && printf '%s\n' "${USERPROFILE}/.codex"
      ;;
    claude)
      [[ -n "${HOME:-}" ]] && printf '%s\n' "${HOME}/.claude"
      [[ -n "${USERPROFILE:-}" ]] && printf '%s\n' "${USERPROFILE}/.claude"
      ;;
  esac
}

dorkpipe_orchestrate_host_config_candidates() {
  local provider="${1:?provider}"
  local host_file
  host_file="$(dockpipe scope resolver "${provider}" config-file 2>/dev/null || true)"
  [[ -n "${host_file}" ]] && printf '%s\n' "${host_file}"
  case "${provider}" in
    claude)
      [[ -n "${HOME:-}" ]] && printf '%s\n' "${HOME}/.claude.json"
      [[ -n "${USERPROFILE:-}" ]] && printf '%s\n' "${USERPROFILE}/.claude.json"
      ;;
  esac
}

dorkpipe_orchestrate_host_auth_dir_for_mount() {
  local provider="${1:?provider}"
  local dir
  while IFS= read -r dir; do
    [[ -n "${dir}" ]] || continue
    case "${provider}" in
      codex)
        [[ -s "${dir}/auth.json" ]] && printf '%s\n' "${dir}" && return 0
        ;;
      claude)
        [[ -s "${dir}/.credentials.json" ]] && printf '%s\n' "${dir}" && return 0
        if compgen -G "${dir}/backups/.claude.json.backup.*" >/dev/null; then
          printf '%s\n' "${dir}"
          return 0
        fi
        ;;
    esac
  done < <(dorkpipe_orchestrate_host_auth_candidates "${provider}")
  return 1
}

dorkpipe_orchestrate_host_config_file_for_mount() {
  local provider="${1:?provider}"
  local file
  while IFS= read -r file; do
    [[ -n "${file}" ]] || continue
    [[ -s "${file}" ]] && printf '%s\n' "${file}" && return 0
  done < <(dorkpipe_orchestrate_host_config_candidates "${provider}")
  return 1
}

dorkpipe_orchestrate_container_auth_mount() {
  local provider="${1:?provider}"
  local host_dir container_dir mode
  host_dir="$(dorkpipe_orchestrate_host_auth_dir_for_mount "${provider}")" || return 1
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
  host_dir="$(dorkpipe_orchestrate_host_auth_dir_for_mount "${provider}")" || return 1
  [[ -n "${host_dir}" && -d "${host_dir}" ]] || return 1
  host_dir="$(dorkpipe_orchestrate_cli_mount_host_path "${host_dir}")"
  printf '%s:/dockpipe-auth/%s:ro\n' "${host_dir}" "${provider}"
}

dorkpipe_orchestrate_container_skills_dir() {
  local provider="${1:?provider}"
  local upper override host_dir
  upper="$(printf '%s' "${provider}" | tr '[:lower:]' '[:upper:]')"
  override="DORKPIPE_ORCH_${upper}_SKILLS_DIR"
  if [[ -n "${!override:-}" ]]; then
    printf '%s\n' "${!override}"
    return 0
  fi
  if [[ -n "${DORKPIPE_ORCH_SKILLS_DIR:-}" ]]; then
    printf '%s\n' "${DORKPIPE_ORCH_SKILLS_DIR}"
    return 0
  fi
  host_dir="$(dorkpipe_orchestrate_host_auth_dir_for_mount "${provider}")" || return 1
  printf '%s\n' "${host_dir}/skills"
}

dorkpipe_orchestrate_container_skills_stage_dir() {
  local provider="${1:?provider}"
  printf '%s\n' "${DORKPIPE_ORCH_ROOT}/skills/${provider}"
}

dorkpipe_orchestrate_skills_render_bin() {
  local render_bin dockpipe_bin
  if [[ -n "${DOCKPIPE_SKILLS_RENDER_BIN:-}" && -x "${DOCKPIPE_SKILLS_RENDER_BIN}" ]]; then
    printf '%s\n' "${DOCKPIPE_SKILLS_RENDER_BIN}"
    return 0
  fi
  render_bin="${DOCKPIPE_ASSETS_DIR:-}/tooling/bin/$(case "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" in Windows_NT:*|*:msys*:*|*:cygwin*:*|*:*:MINGW*) printf 'windows' ;; darwin*:*|*:darwin*:* ) printf 'darwin' ;; *) printf 'linux' ;; esac)/skills-render$(case "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" in Windows_NT:*|*:msys*:*|*:cygwin*:*|*:*:MINGW*) printf '.exe' ;; *) printf '' ;; esac)"
  if [[ -x "${render_bin}" ]]; then
    printf '%s\n' "${render_bin}"
    return 0
  fi
  render_bin="$(dockpipe_sdk require tooling-bin skills-render 2>/dev/null || true)"
  if [[ -n "${render_bin}" && -x "${render_bin}" ]]; then
    printf '%s\n' "${render_bin}"
    return 0
  fi
  dockpipe_bin="$(dorkpipe_orchestrate_dockpipe_bin)" || return 1
  "${dockpipe_bin}" package build source --workdir "${ROOT}" --only dorkpipe >/dev/null
  render_bin="$(dockpipe_sdk require tooling-bin skills-render 2>/dev/null || true)"
  [[ -n "${render_bin}" && -x "${render_bin}" ]] || return 1
  printf '%s\n' "${render_bin}"
}

dorkpipe_orchestrate_render_curated_skills() {
  local provider="${1:?provider}"
  local output_dir="${2:?output dir}"
  local render_bin
  render_bin="$(dorkpipe_orchestrate_skills_render_bin)" || return 1
  mkdir -p "${output_dir}"
  "${render_bin}" --target "${provider}" --output "${output_dir}" --force >/dev/null
}

dorkpipe_orchestrate_prepare_container_skills_dir() {
  local provider="${1:?provider}"
  local stage_dir source_dir stamp_file
  stage_dir="$(dorkpipe_orchestrate_container_skills_stage_dir "${provider}")"
  stamp_file="${stage_dir}/.dorkpipe-orch-skills-ready"
  if [[ -f "${stamp_file}" ]]; then
    printf '%s\n' "${stage_dir}"
    return 0
  fi
  rm -rf "${stage_dir}"
  mkdir -p "${stage_dir}"
  source_dir="$(dorkpipe_orchestrate_container_skills_dir "${provider}" 2>/dev/null || true)"
  if [[ -n "${source_dir}" && -d "${source_dir}" ]]; then
    cp -a "${source_dir}/." "${stage_dir}/" 2>/dev/null || true
  fi
  dorkpipe_orchestrate_render_curated_skills "${provider}" "${stage_dir}" || return 1
  printf 'provider=%s\nsource=%s\n' "${provider}" "${source_dir:-}" > "${stamp_file}"
  printf '%s\n' "${stage_dir}"
}

dorkpipe_orchestrate_container_skills_mount() {
  local provider="${1:?provider}"
  local mode host_dir
  mode="$(printf '%s' "${DORKPIPE_ORCH_CONTAINER_SKILLS:-auto}" | tr '[:upper:]' '[:lower:]')"
  case "${mode}" in
    0|false|no|off|never|none|disabled) return 1 ;;
  esac
  host_dir="$(dorkpipe_orchestrate_prepare_container_skills_dir "${provider}")" || return 1
  [[ -n "${host_dir}" && -d "${host_dir}" ]] || return 1
  host_dir="$(dorkpipe_orchestrate_cli_mount_host_path "${host_dir}")"
  printf '%s:/dockpipe-auth/%s-skills:ro\n' "${host_dir}" "${provider}"
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
      host_file="$(dorkpipe_orchestrate_host_config_file_for_mount "${provider}" || true)"
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

dorkpipe_orchestrate_container_tool_packages() {
  printf '%s\n' "${DORKPIPE_ORCH_CONTAINER_IMAGE_PACKAGES:-${DORKPIPE_ORCH_CONTAINER_APT_PACKAGES:-}}"
}

dorkpipe_orchestrate_normalize_package_list() {
  local pkg packages=()
  for pkg in $*; do
    pkg="${pkg#"${pkg%%[![:space:]]*}"}"
    pkg="${pkg%"${pkg##*[![:space:]]}"}"
    [[ -n "${pkg}" ]] || continue
    if [[ ! "${pkg}" =~ ^[A-Za-z0-9][A-Za-z0-9+_.:-]*$ ]]; then
      echo "invalid orchestration image apt package name: ${pkg}" >&2
      return 1
    fi
    packages+=("${pkg}")
  done
  if ((${#packages[@]} == 0)); then
    return 0
  fi
  printf '%s\n' "${packages[@]}" | sort -u | tr '\n' ' ' | sed 's/[[:space:]]*$//'
}

dorkpipe_orchestrate_sha256_token() {
  local input="${1:-}"
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s' "${input}" | sha256sum | awk '{print substr($1,1,12)}'
    return 0
  fi
  if command -v shasum >/dev/null 2>&1; then
    printf '%s' "${input}" | shasum -a 256 | awk '{print substr($1,1,12)}'
    return 0
  fi
  echo "sha256sum or shasum is required to fingerprint orchestration image package overrides" >&2
  return 1
}

dorkpipe_orchestrate_provider_base_image() {
  local provider="${1:?provider}"
  local upper override version versioned latest bare
  upper="$(printf '%s' "${provider}" | tr '[:lower:]' '[:upper:]')"
  override="DORKPIPE_ORCH_${upper}_BASE_IMAGE"
  if [[ -n "${!override:-}" ]]; then
    printf '%s\n' "${!override}"
    return 0
  fi
  if [[ -n "${DORKPIPE_ORCH_CONTAINER_BASE_IMAGE:-}" ]]; then
    printf '%s\n' "${DORKPIPE_ORCH_CONTAINER_BASE_IMAGE}"
    return 0
  fi
  bare="dockpipe-${provider}"
  latest="${bare}:latest"
  version=""
  if [[ -f "${ROOT}/version" ]]; then
    version="$(tr -d '[:space:]' < "${ROOT}/version" 2>/dev/null || true)"
  fi
  if [[ -n "${version}" ]]; then
    versioned="${bare}:${version}"
    if docker image inspect "${versioned}" >/dev/null 2>&1; then
      printf '%s\n' "${versioned}"
      return 0
    fi
  fi
  if docker image inspect "${latest}" >/dev/null 2>&1; then
    printf '%s\n' "${latest}"
    return 0
  fi
  printf '%s\n' "${bare}"
}

dorkpipe_orchestrate_write_tool_image_dockerfile() {
  local provider="${1:?provider}"
  local base_image="${2:?base image}"
  local packages="${3:?packages}"
  local dockerfile="${4:?dockerfile}"
  local needs_microsoft_repo="false"
  local pkg
  for pkg in ${packages}; do
    case "${pkg}" in
      dotnet-*|aspnetcore-*|netstandard-*) needs_microsoft_repo="true" ;;
    esac
  done
  {
    printf '# syntax=docker/dockerfile:1.7\n'
    printf '# Generated by DorkPipe orchestration for %s worker tooling.\n' "${provider}"
    printf 'FROM %s\n\n' "${base_image}"
    printf 'USER root\n'
    if [[ "${needs_microsoft_repo}" == "true" ]]; then
      cat <<'EOF'
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates gnupg wget \
    && wget -O /tmp/packages-microsoft-prod.deb https://packages.microsoft.com/config/debian/12/packages-microsoft-prod.deb \
    && dpkg -i /tmp/packages-microsoft-prod.deb \
    && rm /tmp/packages-microsoft-prod.deb \
    && rm -rf /var/lib/apt/lists/*

EOF
    fi
    printf 'RUN --mount=type=cache,target=/var/cache/apt,sharing=locked --mount=type=cache,target=/var/lib/apt,sharing=locked apt-get update && apt-get install -y --no-install-recommends %s && rm -rf /var/lib/apt/lists/*\n\n' "${packages}"
    printf 'USER root\n'
  } > "${dockerfile}"
}

dorkpipe_orchestrate_tool_image() {
  local provider="${1:?provider}"
  local raw_packages packages base_image token image dir dockerfile stamp
  raw_packages="$(dorkpipe_orchestrate_container_tool_packages)"
  packages="$(dorkpipe_orchestrate_normalize_package_list "${raw_packages}")" || return 1
  if [[ -z "${packages}" ]]; then
    return 1
  fi
  base_image="$(dorkpipe_orchestrate_provider_base_image "${provider}")"
  token="$(dorkpipe_orchestrate_sha256_token "${provider}|${base_image}|${packages}")" || return 1
  image="dockpipe-${provider}-tools:${token}"
  dir="${DORKPIPE_ORCH_ROOT}/images/${provider}-${token}"
  dockerfile="${dir}/Dockerfile"
  stamp="${dir}/image.txt"
  mkdir -p "${dir}"
  if [[ ! -f "${dockerfile}" ]]; then
    dorkpipe_orchestrate_write_tool_image_dockerfile "${provider}" "${base_image}" "${packages}" "${dockerfile}"
  fi
  if ! docker image inspect "${image}" >/dev/null 2>&1; then
    echo "[dorkpipe] orchestration image: building ${image} from ${base_image} with packages: ${packages}" >&2
    DOCKER_BUILDKIT="${DOCKER_BUILDKIT:-1}" docker build -t "${image}" -f "${dockerfile}" "${dir}" >&2
  fi
  {
    printf 'image=%s\n' "${image}"
    printf 'base_image=%s\n' "${base_image}"
    printf 'packages=%s\n' "${packages}"
    printf 'dockerfile=%s\n' "${dockerfile}"
  } > "${stamp}"
  printf '%s\n' "${image}"
}

dorkpipe_orchestrate_work_mode() {
  local mode="${TASK_WORK_MODE:-${DORKPIPE_ORCH_WORK_MODE:-artifact}}"
  mode="$(printf '%s' "${mode}" | tr '[:upper:]' '[:lower:]' | tr '_' '-')"
  case "${mode}" in
    edit|direct|direct-edit|workspace-edit|writable)
      printf 'edit\n'
      ;;
    artifact|artifacts|readonly|read-only|read-only-artifacts|collect|gather)
      printf 'artifact\n'
      ;;
    *)
      printf 'artifact\n'
      ;;
  esac
}

dorkpipe_orchestrate_edit_isolation_mode() {
  local mode="${DORKPIPE_ORCH_EDIT_ISOLATION:-serialized}"
  mode="$(printf '%s' "${mode}" | tr '[:upper:]' '[:lower:]' | tr '_' '-')"
  case "${mode}" in
    serialized|split-volume)
      printf '%s\n' "${mode}"
      ;;
    *)
      printf 'serialized\n'
      ;;
  esac
}

dorkpipe_orchestrate_edit_worker_acquire() {
  local task_id="${1:?task id}"
  local lease_json="${2:?lease json}"
  local dockpipe_bin mode retry_seconds max_wait_seconds start_time elapsed err_file
  dockpipe_bin="$(dorkpipe_orchestrate_dockpipe_bin)" || return 1
  mode="$(dorkpipe_orchestrate_edit_isolation_mode)"
  if [[ "${mode}" != "serialized" ]]; then
    "${dockpipe_bin}" session worker-acquire "${DOCKPIPE_SESSION_ID}" \
      --workdir "${ROOT}" \
      --worker "${task_id}" \
      --role edit \
      --mode "${mode}" \
      --json > "${lease_json}"
    return 0
  fi
  retry_seconds="${DORKPIPE_ORCH_EDIT_LEASE_RETRY_SECONDS:-2}"
  max_wait_seconds="${DORKPIPE_ORCH_EDIT_LEASE_MAX_WAIT_SECONDS:-900}"
  start_time="$(date +%s)"
  err_file="${lease_json}.err"
  while true; do
    if "${dockpipe_bin}" session worker-acquire "${DOCKPIPE_SESSION_ID}" \
      --workdir "${ROOT}" \
      --worker "${task_id}" \
      --role edit \
      --mode "${mode}" \
      --json > "${lease_json}" 2> "${err_file}"; then
      rm -f "${err_file}"
      return 0
    fi
    if ! grep -qi "active worker lease" "${err_file}" 2>/dev/null; then
      cat "${err_file}" >&2 || true
      rm -f "${err_file}"
      return 1
    fi
    elapsed="$(( $(date +%s) - start_time ))"
    if (( elapsed >= max_wait_seconds )); then
      cat "${err_file}" >&2 || true
      echo "[dorkpipe] timed out waiting for serialized edit lease for ${task_id} after ${elapsed}s" >&2
      rm -f "${err_file}"
      return 1
    fi
    echo "[dorkpipe] waiting for serialized edit lease for ${task_id} (${elapsed}s elapsed)" >&2
    sleep "${retry_seconds}"
  done
}

dorkpipe_orchestrate_edit_worker_release() {
  local task_id="${1:?task id}"
  local status="${2:-released}"
  local apply_changes="${3:-false}"
  local dockpipe_bin args=()
  dockpipe_bin="$(dorkpipe_orchestrate_dockpipe_bin)" || return 1
  args=(
    session worker-release "${DOCKPIPE_SESSION_ID}"
    --workdir "${ROOT}"
    --worker "${task_id}"
    --status "${status}"
  )
  if [[ "$(dorkpipe_orchestrate_bool "${apply_changes}")" == "true" ]]; then
    args+=(--apply)
  fi
  "${dockpipe_bin}" "${args[@]}" >/dev/null
}

dorkpipe_orchestrate_append_work_mode_prompt() {
  local prompt_path="${1:?prompt path}"
  local mode marker
  [[ -f "${prompt_path}" ]] || return 0
  marker="## DorkPipe Work Mode:"
  if grep -q "^${marker}" "${prompt_path}" 2>/dev/null; then
    return 0
  fi
  mode="$(dorkpipe_orchestrate_work_mode)"
  {
    printf '\n---\n\n'
    printf '## DorkPipe Work Mode: %s\n\n' "${mode}"
    case "${mode}" in
      edit)
        if [[ -n "${TASK_OUTPUT_PATH:-}" ]]; then
          cat <<EOF
Target file contract:

- You must write exactly this target file: ${TASK_OUTPUT_PATH}
- Do not edit sibling package files or substitute a different output file.
- Return the final content of ${TASK_OUTPUT_PATH} in your response after writing it.
- If you discover issues outside ${TASK_OUTPUT_PATH}, mention them in the response only; do not edit other files.

EOF
        fi
        cat <<'EOF'
You are in direct workspace edit mode.

- You may edit mounted workspace paths only when they are writable by the declared access and mount policy.
- Use normal repo-worker behavior: inspect files, make source changes, and run targeted validation when the task asks for implementation work.
- Keep generated outputs out of source unless the task explicitly asks for source-controlled artifacts.
- Do not commit, push, publish, or apply outside the declared workspace unless the task explicitly asks for that.
- Respect deny rules and secret boundaries even if a mounted filesystem appears writable.
EOF
        ;;
      *)
        cat <<'EOF'
You are in readonly artifact-gathering mode.

- Treat mounted source paths as read-only, even if filesystem permissions appear writable.
- Do not use apply_patch, touch, mkdir, redirection writes, in-place edits, or generated files in mounted source repos.
- Inspect source files and run non-mutating checks only.
- Return the requested artifact content in this response; DorkPipe apply/promote stages write approved files later.
- If the task asks for YAML or markdown content, output only that content and do not narrate attempted file writes.
EOF
        ;;
    esac
    cat <<'EOF'

Tooling note:

- Use available shell tools for inspection and validation.
- If expected tooling such as python, ruby, yq, or language-specific CLIs is missing, state the skipped validation or use a safe available fallback; do not pretend the check ran.
EOF
  } >> "${prompt_path}"
}

dorkpipe_orchestrate_worker_cwd() {
  local provider="${1:-}"
  local upper provider_override cwd
  if [[ -n "${provider}" ]]; then
    upper="$(printf '%s' "${provider}" | tr '[:lower:]' '[:upper:]')"
    provider_override="DORKPIPE_ORCH_${upper}_WORKER_CWD"
  fi
  cwd="${provider_override:+${!provider_override:-}}"
  cwd="${cwd:-${DORKPIPE_ORCH_WORKER_CWD:-${DORKPIPE_ORCH_TARGET_GUEST_PATH:-}}}"
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

dorkpipe_orchestrate_claude_auth_status_ok() {
  command -v claude >/dev/null 2>&1 || return 2
  claude auth status --json 2>/dev/null | grep -Eq '"loggedIn"[[:space:]]*:[[:space:]]*true'
}

dorkpipe_orchestrate_auth_is_available() {
  local provider="${1:?provider}"
  local host_dir host_file
  local -a dirs files
  case "${provider}" in
    codex)
      if [[ -n "${OPENAI_API_KEY:-}" || -n "${CODEX_API_KEY:-}" ]]; then
        return 0
      fi
      host_dir="$(dorkpipe_orchestrate_container_auth_dir "${provider}" 2>/dev/null || true)"
      dirs=()
      [[ -n "${host_dir}" ]] && dirs+=("${host_dir}")
      [[ -n "${HOME:-}" ]] && dirs+=("${HOME}/.codex")
      [[ -n "${USERPROFILE:-}" ]] && dirs+=("${USERPROFILE}/.codex")
      local dir
      for dir in "${dirs[@]}"; do
        [[ -n "${dir}" && -s "${dir}/auth.json" ]] && return 0
      done
      return 1
      ;;
    claude)
      if [[ -n "${ANTHROPIC_API_KEY:-}" || -n "${CLAUDE_API_KEY:-}" ]]; then
        return 0
      fi
      if command -v claude >/dev/null 2>&1; then
        if dorkpipe_orchestrate_claude_auth_status_ok; then
          return 0
        fi
        return 1
      fi
      host_dir="$(dorkpipe_orchestrate_container_auth_dir "${provider}" 2>/dev/null || true)"
      host_file="$(dockpipe scope resolver "${provider}" config-file 2>/dev/null || true)"
      dirs=()
      files=()
      [[ -n "${host_dir}" ]] && dirs+=("${host_dir}")
      [[ -n "${HOME:-}" ]] && dirs+=("${HOME}/.claude")
      [[ -n "${USERPROFILE:-}" ]] && dirs+=("${USERPROFILE}/.claude")
      [[ -n "${host_file}" ]] && files+=("${host_file}")
      [[ -n "${HOME:-}" ]] && files+=("${HOME}/.claude.json")
      [[ -n "${USERPROFILE:-}" ]] && files+=("${USERPROFILE}/.claude.json")
      local dir file
      for dir in "${dirs[@]}"; do
        [[ -n "${dir}" && -s "${dir}/.credentials.json" ]] && return 0
        if compgen -G "${dir}/backups/.claude.json.backup.*" >/dev/null; then
          return 0
        fi
      done
      for file in "${files[@]}"; do
        [[ -n "${file}" && -s "${file}" ]] && return 0
      done
      return 1
      ;;
  esac
  return 0
}

dorkpipe_orchestrate_auth_login_command_text() {
  case "${1:-}" in
    codex) printf 'codex login\n' ;;
    claude) printf 'claude auth login\n' ;;
    *) printf '%s login\n' "${1:-provider}" ;;
  esac
}

dorkpipe_orchestrate_has_tty() {
  { : </dev/tty >/dev/tty; } 2>/dev/null
}

dorkpipe_orchestrate_run_auth_login() {
  local provider="${1:?provider}"
  case "${provider}" in
    codex)
      if ! command -v codex >/dev/null 2>&1; then
        echo "codex login unavailable: codex CLI is not installed on the host PATH" >&2
        return 1
      fi
      echo "[dorkpipe] launching host login: codex login" >&2
      if dorkpipe_orchestrate_has_tty; then
        codex login </dev/tty >/dev/tty
      else
        codex login
      fi
      ;;
    claude)
      if ! command -v claude >/dev/null 2>&1; then
        echo "claude auth login unavailable: claude CLI is not installed on the host PATH" >&2
        return 1
      fi
      echo "[dorkpipe] launching host login: claude auth login" >&2
      if dorkpipe_orchestrate_has_tty; then
        claude auth login </dev/tty >/dev/tty
      else
        claude auth login
      fi
      ;;
    *)
      echo "auth login unsupported for provider: ${provider}" >&2
      return 1
      ;;
  esac
}

dorkpipe_orchestrate_worker_log_shows_auth_failure() {
  local provider="${1:?provider}"
  local log_path="${2:?log path}"
  [[ -f "${log_path}" ]] || return 1
  case "${provider}" in
    codex|claude)
      grep -Eqi 'not logged in|please run /login|please run .*login|authentication failed|auth(entication)? required' "${log_path}"
      ;;
    *)
      return 1
      ;;
  esac
}

dorkpipe_orchestrate_auth_preflight() {
  local provider="${1:?provider}"
  local mode answer command_text
  if dorkpipe_orchestrate_auth_is_available "${provider}"; then
    return 0
  fi
  command_text="$(dorkpipe_orchestrate_auth_login_command_text "${provider}")"
  echo "[dorkpipe] ${provider} auth preflight failed." >&2
  echo "[dorkpipe] Run '${command_text}' or set the provider API key before launching ${provider} workers." >&2
  mode="$(printf '%s' "${DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING:-ask}" | tr '[:upper:]' '[:lower:]')"
  case "${mode}" in
    1|true|yes|always|auto-login)
      ;;
    0|false|no|never|off|disabled)
      return 1
      ;;
    ask|prompt|"")
      if ! dorkpipe_orchestrate_has_tty; then
        echo "[dorkpipe] cannot ask to log in because this run is non-interactive." >&2
        return 1
      fi
      printf '[dorkpipe] Login to %s now? [y/N] ' "${provider}" >/dev/tty
      IFS= read -r answer </dev/tty || answer=""
      case "${answer}" in
        y|Y|yes|YES) ;;
        *)
          echo "[dorkpipe] ${provider} login skipped by user." >&2
          return 1
          ;;
      esac
      ;;
    *)
      echo "[dorkpipe] unknown DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING=${DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING}; expected ask, never, or always" >&2
      return 1
      ;;
  esac
  dorkpipe_orchestrate_run_auth_login "${provider}" || return 1
  if dorkpipe_orchestrate_auth_is_available "${provider}"; then
    echo "[dorkpipe] ${provider} auth preflight passed after login." >&2
    return 0
  fi
  echo "[dorkpipe] ${provider} auth still unavailable after login. Check the login result and retry." >&2
  return 1
}

dorkpipe_orchestrate_auth_recover_after_worker_failure() {
  local provider="${1:?provider}"
  local log_path="${2:?log path}"
  local mode answer command_text
  dorkpipe_orchestrate_worker_log_shows_auth_failure "${provider}" "${log_path}" || return 1
  if [[ -s "${log_path}" ]]; then
    cat "${log_path}" >&2
  fi
  command_text="$(dorkpipe_orchestrate_auth_login_command_text "${provider}")"
  echo "[dorkpipe] ${provider} worker failed because host auth appears to be missing or expired." >&2
  echo "[dorkpipe] Run '${command_text}' on the host, then retry this worker." >&2
  mode="$(printf '%s' "${DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING:-ask}" | tr '[:upper:]' '[:lower:]')"
  case "${mode}" in
    1|true|yes|always|auto-login)
      ;;
    0|false|no|never|off|disabled)
      return 1
      ;;
    ask|prompt|"")
      if ! dorkpipe_orchestrate_has_tty; then
        echo "[dorkpipe] cannot recover ${provider} auth interactively because this run is non-interactive." >&2
        return 1
      fi
      printf '[dorkpipe] Login to %s on the host now, then retry this worker? [y/N] ' "${provider}" >/dev/tty
      IFS= read -r answer </dev/tty || answer=""
      case "${answer}" in
        y|Y|yes|YES) ;;
        *)
          echo "[dorkpipe] ${provider} auth recovery skipped by user." >&2
          return 1
          ;;
      esac
      ;;
    *)
      echo "[dorkpipe] unknown DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING=${DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING}; expected ask, never, or always" >&2
      return 1
      ;;
  esac
  dorkpipe_orchestrate_run_auth_login "${provider}" || return 1
  if dorkpipe_orchestrate_auth_is_available "${provider}"; then
    echo "[dorkpipe] ${provider} auth recovery succeeded; retrying worker once." >&2
    return 0
  fi
  echo "[dorkpipe] ${provider} auth still unavailable after host login. Worker retry skipped." >&2
  return 1
}

dorkpipe_orchestrate_run_container_worker() {
  local provider="${1:?provider}"
  local prompt_path="${2:?prompt path}"
  local response_path="${3:?response path}"
  local selected_model="${4:-}"
  local dockpipe_bin auth_mount raw_response_path raw_error_path worker_cwd tool_image
  local lease_json lease_acquired lease_apply_on_release worker_session_volume
  local rc release_status auth_retry_performed
  dockpipe_bin="$(dorkpipe_orchestrate_dockpipe_bin)" || return 1
  raw_response_path="${response_path}.raw"
  raw_error_path="${response_path}.err"
  worker_cwd="$(dorkpipe_orchestrate_worker_cwd "${provider}")"
  dorkpipe_orchestrate_auth_preflight "${provider}" || return 1
  lease_json="${response_path}.lease.json"
  lease_acquired="false"
  lease_apply_on_release="false"
  worker_session_volume=""
  auth_retry_performed="false"

  if [[ "$(dorkpipe_orchestrate_work_mode)" == "edit" && -n "${DOCKPIPE_SESSION_ID:-}" && -n "${DOCKPIPE_SESSION_VOLUME:-}" ]]; then
    dorkpipe_orchestrate_edit_worker_acquire "${task_id:-worker}" "${lease_json}" || return 1
    eval "$("$(dorkpipe_orchestrate_helper_bin)" worker-lease-env "${lease_json}")"
    lease_acquired="true"
    worker_session_volume="${LEASE_VOLUME:-}"
    if [[ "${LEASE_MODE:-}" == "split-volume" ]]; then
      lease_apply_on_release="true"
    fi
  fi

  local args=(
    "--workdir" "${ROOT}"
    "--runtime" "dockerimage"
    "--resolver" "${provider}"
    "--no-data"
    "--env" "HOME=/home/node"
    "--env" "PATH=/usr/local/bin:/usr/bin:/bin:/usr/local/games:/usr/games"
    "--env" "DORKPIPE_ORCH_WORK_MODE=$(dorkpipe_orchestrate_work_mode)"
  )
  if dorkpipe_orchestrate_is_cloud_provider "${provider}"; then
    # Nested dockpipe runs can inherit the parent workflow's offline Docker network
    # policy via DOCKPIPE_DOCKER_NETWORK=none. Cloud workers need real egress.
    args+=("--env" "DOCKPIPE_DOCKER_NETWORK=bridge")
  fi
  if [[ -n "$(dorkpipe_orchestrate_container_tool_packages | tr -d '[:space:]')" ]]; then
    tool_image="$(dorkpipe_orchestrate_tool_image "${provider}")" || return 1
    args+=("--isolate" "${tool_image}")
  fi
  while IFS= read -r auth_env; do
    [[ -n "${auth_env}" ]] || continue
    args+=("--env" "${auth_env}")
  done < <(dorkpipe_orchestrate_container_auth_envs "${provider}")
  if auth_mount="$(dorkpipe_orchestrate_container_auth_seed_mount "${provider}" 2>/dev/null)"; then
    args+=("--mount" "${auth_mount}")
  fi
  if auth_mount="$(dorkpipe_orchestrate_container_skills_mount "${provider}" 2>/dev/null)"; then
    args+=("--mount" "${auth_mount}")
  fi
  if [[ -n "${worker_session_volume}" ]]; then
    args+=("--env" "DOCKPIPE_SESSION_VOLUME=${worker_session_volume}")
    args+=("--env" "DOCKPIPE_SESSION_VOLUME_AUTHORITATIVE=1")
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
      while :; do
        rm -f "${raw_error_path}"
        set +e
        MSYS2_ARG_CONV_EXCL='*' "${dockpipe_bin}" "${args[@]}" -- \
          bash -c '
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
          if [[ -d /dockpipe-auth/codex-skills ]]; then
            mkdir -p /home/node/.codex/skills
            cp -a /dockpipe-auth/codex-skills/. /home/node/.codex/skills/ 2>/dev/null || true
            chmod -R u+rwX /home/node/.codex/skills 2>/dev/null || true
          fi
          if [[ -n "${2:-}" && "${2:-}" != "cli" ]]; then
            codex exec --dangerously-bypass-approvals-and-sandbox --model "$2" "$1" </dev/null
          else
            codex exec --dangerously-bypass-approvals-and-sandbox "$1" </dev/null
          fi
          ' _ "$(cat "${prompt_path}")" "${selected_model}" "${worker_cwd}" > "${raw_response_path}" 2> "${raw_error_path}"
        rc=$?
        set -e
        if [[ "${rc:-0}" -eq 0 ]]; then
          break
        fi
        if [[ "${auth_retry_performed}" != "true" ]] && dorkpipe_orchestrate_auth_recover_after_worker_failure "${provider}" "${raw_error_path}"; then
          auth_retry_performed="true"
          continue
        fi
        [[ -s "${raw_error_path}" ]] && cat "${raw_error_path}" >&2
        break
      done
      ;;
    claude)
      while :; do
        rm -f "${raw_error_path}"
        set +e
        MSYS2_ARG_CONV_EXCL='*' "${dockpipe_bin}" "${args[@]}" -- \
          bash -c '
          set -euo pipefail
          export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin${PATH:+:$PATH}"
          if [[ -n "${3:-}" ]]; then
            cd "$3"
          fi
          mkdir -p /home/node/.claude
          if [[ -d /dockpipe-auth/claude ]]; then
            shopt -s dotglob nullglob
            for item in /dockpipe-auth/claude/*; do
              name="$(basename "${item}")"
              case "${name}" in
                cache|debug|downloads|sessions|history.jsonl) continue ;;
              esac
              cp -a "${item}" /home/node/.claude/ 2>/dev/null || true
            done
            shopt -u dotglob nullglob
            chmod -R u+rwX /home/node/.claude 2>/dev/null || true
          fi
          if [[ -f /dockpipe-auth/claude-config/.claude.json ]]; then
            cp /dockpipe-auth/claude-config/.claude.json /home/node/.claude.json 2>/dev/null || true
            chmod u+rw /home/node/.claude.json 2>/dev/null || true
          fi
          if [[ ! -f /home/node/.claude.json && -d /home/node/.claude/backups ]]; then
            latest="$(find /home/node/.claude/backups -maxdepth 1 -type f -name ".claude.json.backup.*" -printf "%T@ %p\n" 2>/dev/null | sort -nr | head -1 | cut -d" " -f2-)"
            if [[ -n "${latest:-}" ]]; then
              cp "${latest}" /home/node/.claude.json
            fi
          fi
          if [[ -f /home/node/.claude.json && -n "${3:-}" ]]; then
            node - "$3" <<'"'"'NODE'"'"'
const fs = require("fs");

const guestProjectRoot = process.argv[2];
if (!guestProjectRoot) {
  process.exit(0);
}

const configPath = "/home/node/.claude.json";
let payload;
try {
  payload = JSON.parse(fs.readFileSync(configPath, "utf8"));
} catch {
  process.exit(0);
}

if (!payload || typeof payload !== "object") {
  process.exit(0);
}

if (!payload.projects || typeof payload.projects !== "object" || Array.isArray(payload.projects)) {
  payload.projects = {};
}

const existing = payload.projects[guestProjectRoot];
const project = existing && typeof existing === "object" && !Array.isArray(existing) ? existing : {};
if (!Array.isArray(project.allowedTools)) {
  project.allowedTools = [];
}
if (!Array.isArray(project.mcpContextUris)) {
  project.mcpContextUris = [];
}
if (!Array.isArray(project.enabledMcpjsonServers)) {
  project.enabledMcpjsonServers = [];
}
if (!Array.isArray(project.disabledMcpjsonServers)) {
  project.disabledMcpjsonServers = [];
}
project.hasTrustDialogAccepted = true;
if (typeof project.projectOnboardingSeenCount !== "number") {
  project.projectOnboardingSeenCount = 0;
}
if (typeof project.hasClaudeMdExternalIncludesApproved !== "boolean") {
  project.hasClaudeMdExternalIncludesApproved = false;
}
if (typeof project.hasClaudeMdExternalIncludesWarningShown !== "boolean") {
  project.hasClaudeMdExternalIncludesWarningShown = false;
}
payload.projects[guestProjectRoot] = project;

fs.writeFileSync(configPath, JSON.stringify(payload, null, 2));
NODE
          fi
          if [[ -d /dockpipe-auth/claude-skills ]]; then
            mkdir -p /home/node/.claude/skills
            cp -a /dockpipe-auth/claude-skills/. /home/node/.claude/skills/ 2>/dev/null || true
            chmod -R u+rwX /home/node/.claude/skills 2>/dev/null || true
          fi
          if [[ -n "${2:-}" && "${2:-}" != "cli" ]]; then
            claude --dangerously-skip-permissions --model "$2" -p "$1" </dev/null
          else
            claude --dangerously-skip-permissions -p "$1" </dev/null
          fi
          ' _ "$(cat "${prompt_path}")" "${selected_model}" "${worker_cwd}" > "${raw_response_path}" 2> "${raw_error_path}"
        rc=$?
        set -e
        if [[ "${rc:-0}" -eq 0 ]]; then
          break
        fi
        if [[ "${auth_retry_performed}" != "true" ]] && dorkpipe_orchestrate_auth_recover_after_worker_failure "${provider}" "${raw_error_path}"; then
          auth_retry_performed="true"
          continue
        fi
        [[ -s "${raw_error_path}" ]] && cat "${raw_error_path}" >&2
        break
      done
      ;;
    *)
      set -e
      return 1
      ;;
  esac
  set -e
  release_status="released"
  if [[ "${rc:-0}" -ne 0 ]]; then
    release_status="failed"
  fi
  if [[ "${lease_acquired}" == "true" ]]; then
    dorkpipe_orchestrate_edit_worker_release "${task_id:-worker}" "${release_status}" "${lease_apply_on_release}" || return 1
    rm -f "${lease_json}"
  fi
  if [[ "${rc:-0}" -ne 0 ]]; then
    return "${rc}"
  fi
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
  local attempts=0 stale_seconds now lock_mtime
  stale_seconds="${DORKPIPE_ORCH_CLOUD_USAGE_LOCK_STALE_SECONDS:-30}"
  local status=0
  until mkdir "${lock_dir}" 2>/dev/null; do
    attempts="$((attempts + 1))"
    if [[ -d "${lock_dir}" ]]; then
      now="$(date +%s)"
      lock_mtime="$(stat -c %Y "${lock_dir}" 2>/dev/null || printf '0')"
      if [[ "${lock_mtime}" =~ ^[0-9]+$ ]] && (( lock_mtime > 0 )) && (( now - lock_mtime > stale_seconds )); then
        echo "removing stale cloud usage lock: ${lock_dir}" >&2
        rmdir "${lock_dir}" 2>/dev/null || true
        continue
      fi
    fi
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
