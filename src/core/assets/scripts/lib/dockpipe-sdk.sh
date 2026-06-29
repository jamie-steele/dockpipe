#!/usr/bin/env bash

dockpipe_repo_root() {
  local root="${1:-}"
  if [[ -z "$root" ]]; then
    root="${DOCKPIPE_WORKDIR:-$(pwd)}"
  fi
  (cd "$root" && pwd)
}

dockpipe_is_windows_host() {
  case "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" in
    Windows_NT:*|*:msys*:*|*:cygwin*:*|*:*:MINGW*)
      return 0
      ;;
  esac
  return 1
}

dockpipe_repo_binary_candidate() {
  local root="$1"
  local relative="$2"
  local candidate="$root/$relative"
  if [[ -x "$candidate" ]]; then
    printf '%s\n' "$candidate"
    return 0
  fi
  if dockpipe_is_windows_host && [[ -f "$candidate.exe" ]]; then
    printf '%s\n' "$candidate.exe"
    return 0
  fi
  return 1
}

dockpipe_resolve_dockpipe_bin() {
  local root
  root="$(dockpipe_repo_root "${1:-}")"
  if [[ -n "${DOCKPIPE_BIN:-}" ]]; then
    printf '%s\n' "$DOCKPIPE_BIN"
    return 0
  fi
  local candidate
  if candidate="$(dockpipe_repo_binary_candidate "$root" "src/bin/dockpipe" 2>/dev/null)"; then
    printf '%s\n' "$candidate"
    return 0
  fi
  command -v dockpipe 2>/dev/null || return 1
}

dockpipe_sdk_refresh() {
  local root="${1:-}"
  local resolved_root resolved_dockpipe resolved_workflow_name resolved_script_dir resolved_assets_dir resolved_package_root resolved_state_dir resolved_package_id resolved_package_state_dir resolved_artifact_root resolved_output_root
  resolved_root="$(dockpipe_repo_root "$root")"
  resolved_dockpipe="$(dockpipe_resolve_dockpipe_bin "$resolved_root" 2>/dev/null || true)"
  resolved_workflow_name="${DOCKPIPE_WORKFLOW_NAME:-}"
  resolved_script_dir="$(dockpipe_script_dir)"
  resolved_assets_dir="$(dockpipe_find_assets_dir "$resolved_script_dir")"
  resolved_package_root="$(dockpipe_find_package_root "$resolved_script_dir")"
  resolved_state_dir="${DOCKPIPE_STATE_DIR:-$resolved_root/bin/.dockpipe}"
  resolved_artifact_root="${DOCKPIPE_ARTIFACT_ROOT:-}"
  if [[ -z "$resolved_artifact_root" ]]; then
    resolved_artifact_root="$resolved_state_dir/workflows/$(__dockpipe_sdk_sanitize_workflow_scope "${resolved_workflow_name:-default}")/artifacts"
  fi
  resolved_output_root="${DOCKPIPE_OUTPUT_ROOT:-$resolved_artifact_root}"
  resolved_package_id="${DOCKPIPE_PACKAGE_ID:-}"
  if [[ -z "$resolved_package_id" && -n "$resolved_package_root" ]]; then
    resolved_package_id="$(basename "$resolved_package_root")"
  fi
  resolved_package_id="$(__dockpipe_sdk_sanitize_scope "${resolved_package_id:-default}")"
  resolved_package_state_dir="${DOCKPIPE_PACKAGE_STATE_DIR:-$resolved_state_dir/packages/$resolved_package_id}"

  declare -gA dockpipe
  dockpipe=()
  dockpipe[workdir]="$resolved_root"
  dockpipe[dockpipe_bin]="$resolved_dockpipe"
  dockpipe[workflow_name]="$resolved_workflow_name"
  dockpipe[script_dir]="$resolved_script_dir"
  dockpipe[assets_dir]="$resolved_assets_dir"
  dockpipe[package_root]="$resolved_package_root"
  dockpipe[state_dir]="$resolved_state_dir"
  dockpipe[artifact_root]="$resolved_artifact_root"
  dockpipe[output_root]="$resolved_output_root"
  dockpipe[package_id]="$resolved_package_id"
  dockpipe[package_state_dir]="$resolved_package_state_dir"

  export DOCKPIPE_SDK_ROOT="$resolved_root"
  export DOCKPIPE_WORKDIR="$resolved_root"
  if [[ -n "$resolved_dockpipe" ]]; then
    export DOCKPIPE_BIN="$resolved_dockpipe"
  fi
  if [[ -n "$resolved_workflow_name" ]]; then
    export DOCKPIPE_WORKFLOW_NAME="$resolved_workflow_name"
  fi
  if [[ -n "$resolved_script_dir" ]]; then
    export DOCKPIPE_SCRIPT_DIR="$resolved_script_dir"
  fi
  if [[ -n "$resolved_assets_dir" ]]; then
    export DOCKPIPE_ASSETS_DIR="$resolved_assets_dir"
  fi
  if [[ -n "$resolved_package_root" ]]; then
    export DOCKPIPE_PACKAGE_ROOT="$resolved_package_root"
  fi
  if [[ -n "$resolved_state_dir" ]]; then
    export DOCKPIPE_STATE_DIR="$resolved_state_dir"
  fi
  if [[ -n "$resolved_artifact_root" ]]; then
    export DOCKPIPE_ARTIFACT_ROOT="$resolved_artifact_root"
  fi
  if [[ -n "$resolved_output_root" ]]; then
    export DOCKPIPE_OUTPUT_ROOT="$resolved_output_root"
  fi
  if [[ -n "$resolved_package_id" ]]; then
    export DOCKPIPE_PACKAGE_ID="$resolved_package_id"
  fi
  if [[ -n "$resolved_package_state_dir" ]]; then
    export DOCKPIPE_PACKAGE_STATE_DIR="$resolved_package_state_dir"
  fi
  dockpipe_sdk_apply_artifact_bindings
}

dockpipe_script_dir() {
  local sdk_source="${BASH_SOURCE[0]}"
  local source_path=""
  local idx
  for ((idx = 1; idx < ${#BASH_SOURCE[@]}; idx++)); do
    if [[ "${BASH_SOURCE[$idx]}" != "$sdk_source" ]]; then
      source_path="${BASH_SOURCE[$idx]}"
      break
    fi
  done
  if [[ -z "$source_path" ]]; then
    source_path="${BASH_SOURCE[${#BASH_SOURCE[@]}-1]:-${BASH_SOURCE[0]}}"
  fi
  (cd "$(dirname "$source_path")" && pwd)
}

dockpipe_find_assets_dir() {
  local start="${1:-}"
  [[ -n "$start" ]] || return 0
  local current="$start"
  while [[ -n "$current" && "$current" != "/" ]]; do
    if [[ "$(basename "$current")" == "assets" ]]; then
      printf '%s\n' "$current"
      return 0
    fi
    current="$(dirname "$current")"
  done
  return 0
}

dockpipe_find_package_root() {
  local start="${1:-}"
  [[ -n "$start" ]] || return 0
  local current="$start"
  while [[ -n "$current" && "$current" != "/" ]]; do
    if [[ -f "$current/package.yml" ]]; then
      printf '%s\n' "$current"
      return 0
    fi
    current="$(dirname "$current")"
  done
  return 0
}

dockpipe_sdk_get() {
  local field="${1:-}"
  case "$field" in
    workdir|dockpipe_bin|workflow_name|script_dir|package_root|assets_dir|state_dir|artifact_root|output_root|package_id|package_state_dir)
      printf '%s\n' "${dockpipe[$field]:-}"
      ;;
    *)
      echo "dockpipe sdk: unknown field ${field:-<empty>}" >&2
      return 1
      ;;
  esac
}

__dockpipe_sdk_sanitize_scope() {
  local scope="${1:-}"
  scope="${scope,,}"
  local out="" ch last_dash=0 i
  for ((i = 0; i < ${#scope}; i++)); do
    ch="${scope:i:1}"
    case "$ch" in
      [a-z0-9])
        out+="$ch"
        last_dash=0
        ;;
      [._/\ -])
        if [[ "$last_dash" -eq 0 ]]; then
          out+="-"
          last_dash=1
        fi
        ;;
      *)
        if [[ "$last_dash" -eq 0 ]]; then
          out+="-"
          last_dash=1
        fi
        ;;
    esac
  done
  out="${out#-}"
  out="${out%-}"
  printf '%s\n' "${out:-default}"
}

__dockpipe_sdk_sanitize_workflow_scope() {
  local scope="${1:-}"
  local out="" ch last_dash=0 i
  for ((i = 0; i < ${#scope}; i++)); do
    ch="${scope:i:1}"
    case "$ch" in
      [A-Za-z0-9._-])
        out+="$ch"
        last_dash=0
        ;;
      [/\ ])
        if [[ "$last_dash" -eq 0 ]]; then
          out+="-"
          last_dash=1
        fi
        ;;
      *)
        if [[ "$last_dash" -eq 0 ]]; then
          out+="-"
          last_dash=1
        fi
        ;;
    esac
  done
  out="${out#-}"
  out="${out%-}"
  printf '%s\n' "${out:-default}"
}

__dockpipe_sdk_join_path() {
  local base="${1:?base path}"
  shift || true
  local part
  printf '%s' "$base"
  for part in "$@"; do
    [[ -n "$part" ]] || continue
    printf '/%s' "${part#/}"
  done
  printf '\n'
}

dockpipe_sdk_state_dir() {
  if [[ -n "${DOCKPIPE_STATE_DIR:-}" ]]; then
    printf '%s\n' "$DOCKPIPE_STATE_DIR"
    return 0
  fi
  printf '%s/bin/.dockpipe\n' "$(dockpipe_sdk_get workdir)"
}

dockpipe_sdk_build_dir() {
  __dockpipe_sdk_join_path "$(dockpipe_sdk_state_dir)/build" "$@"
}

dockpipe_sdk_package_state_dir() {
  local scope="${1:-}"
  shift || true
  if [[ -z "$scope" && -n "${DOCKPIPE_PACKAGE_STATE_DIR:-}" ]]; then
    __dockpipe_sdk_join_path "$DOCKPIPE_PACKAGE_STATE_DIR" "$@"
    return 0
  fi
  if [[ -z "$scope" ]]; then
    scope="${DOCKPIPE_PACKAGE_ID:-${dockpipe[package_id]:-}}"
  fi
  if [[ -z "$scope" && -n "${dockpipe[package_root]:-}" ]]; then
    scope="$(basename "${dockpipe[package_root]}")"
  fi
  scope="$(__dockpipe_sdk_sanitize_scope "${scope:-default}")"
  __dockpipe_sdk_join_path "$(dockpipe_sdk_state_dir)/packages/$scope" "$@"
}

dockpipe_sdk_workflow_state_dir() {
  local scope="${1:-}"
  shift || true
  if [[ -z "$scope" ]]; then
    scope="${DOCKPIPE_WORKFLOW_NAME:-${dockpipe[workflow_name]:-default}}"
  fi
  scope="$(__dockpipe_sdk_sanitize_workflow_scope "${scope:-default}")"
  __dockpipe_sdk_join_path "$(dockpipe_sdk_state_dir)/workflows/$scope" "$@"
}

dockpipe_sdk_scope() {
  local bin
  bin="$(dockpipe_sdk_get dockpipe_bin)"
  if [[ -z "$bin" ]]; then
    echo "dockpipe sdk: dockpipe binary not found; set DOCKPIPE_BIN or add dockpipe to PATH" >&2
    return 1
  fi
  "$bin" scope --workdir "$(dockpipe_sdk_get workdir)" "$@"
}

dockpipe_sdk_ci_artifact_dir() {
  local kind="${1:-}"
  local binding="${2:-${DOCKPIPE_CI_ARTIFACT_SCOPE:-}}"
  if [[ -z "$binding" && -n "${DOCKPIPE_WORKFLOW_NAME:-${dockpipe[workflow_name]:-}}" ]]; then
    binding="workflow"
  fi
  local env_name="" package_suffix="" workflow_suffix=""
  case "$kind" in
    raw)
      env_name="DOCKPIPE_CI_RAW_DIR"
      package_suffix="raw"
      workflow_suffix="ci-raw"
      ;;
    analysis)
      env_name="DOCKPIPE_CI_ANALYSIS_DIR"
      package_suffix="analysis"
      workflow_suffix="ci-analysis"
      ;;
    *)
      echo "dockpipe sdk: unknown ci artifact kind ${kind:-<empty>}" >&2
      return 1
      ;;
  esac

  local configured="${!env_name:-}"
  if [[ -n "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi

  case "$binding" in
    workflow)
      dockpipe_sdk_scope artifacts "$workflow_suffix"
      ;;
    workflow:*)
      dockpipe_sdk_scope workflow "${binding#workflow:}" "$workflow_suffix"
      ;;
    package|package:dorkpipe|"")
      dockpipe_sdk_package_state_dir dorkpipe ci "$package_suffix"
      ;;
    package:*)
      dockpipe_sdk_package_state_dir "${binding#package:}" ci "$package_suffix"
      ;;
    *)
      dockpipe_sdk_scope workflow "$binding" "$workflow_suffix"
      ;;
  esac
}

dockpipe_sdk_bind_ci_artifacts() {
  local binding="${1:-${DOCKPIPE_CI_ARTIFACT_SCOPE:-}}"
  export DOCKPIPE_CI_RAW_DIR
  export DOCKPIPE_CI_ANALYSIS_DIR
  DOCKPIPE_CI_RAW_DIR="$(dockpipe_sdk_ci_artifact_dir raw "$binding")"
  DOCKPIPE_CI_ANALYSIS_DIR="$(dockpipe_sdk_ci_artifact_dir analysis "$binding")"
}

dockpipe_sdk_apply_artifact_bindings() {
  export DOCKPIPE_CI_RAW_DIR="${DOCKPIPE_CI_RAW_DIR:-$(dockpipe_sdk_ci_artifact_dir raw)}"
  export DOCKPIPE_CI_ANALYSIS_DIR="${DOCKPIPE_CI_ANALYSIS_DIR:-$(dockpipe_sdk_ci_artifact_dir analysis)}"
}

dockpipe_workflow_name() {
  if [[ -n "${dockpipe[workflow_name]:-}" ]]; then
    printf '%s\n' "${dockpipe[workflow_name]}"
    return 0
  fi
  return 1
}

dockpipe_require_dockpipe_bin() {
  local bin="${DOCKPIPE_BIN:-${dockpipe[dockpipe_bin]:-}}"
  if [[ -z "$bin" ]]; then
    echo "dockpipe sdk: dockpipe binary not found; set DOCKPIPE_BIN or add dockpipe to PATH" >&2
    return 1
  fi
  printf '%s\n' "$bin"
}

__dockpipe_sdk_fatal() {
  local prefix="${WF_NS:-${dockpipe[workflow_name]:-dockpipe}}"
  echo "${prefix}: $*" >&2
  exit 1
}

__dockpipe_sdk_json_escape() {
  local value="${1-}"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '%s' "$value"
}

__dockpipe_sdk_truthy() {
  case "${1:-}" in
    1|true|TRUE|True|yes|YES|Yes|y|Y|on|ON|On)
      return 0
      ;;
  esac
  return 1
}

__dockpipe_sdk_prompt_mode() {
  case "${DOCKPIPE_SDK_PROMPT_MODE:-}" in
    json)
      printf 'json\n'
      return 0
      ;;
    terminal)
      printf 'terminal\n'
      return 0
      ;;
  esac
  if [[ -t 0 && -t 2 ]]; then
    printf 'terminal\n'
    return 0
  fi
  printf 'noninteractive\n'
}

__dockpipe_sdk_emit_prompt_event() {
  local prompt_type="$1"
  local prompt_id="$2"
  local title="$3"
  local message="$4"
  local default_value="$5"
  local sensitive="$6"
  local intent="$7"
  local automation_group="$8"
  local allow_auto_approve="$9"
  local auto_approve_value="${10}"
  local path_mode="${11:-}"
  local file_filter="${12:-}"
  local must_exist="${13:-false}"
  local base_dir="${14:-${DOCKPIPE_WORKDIR:-$(pwd)}}"
  local resource_mode="${15:-select}"
  local resource_selection="${16:-single}"
  local resource_kind="${17:-file}"
  local filters_raw="${18:-}"
  shift 18 || true
  local options=("$@")
  local options_json="" opt filters_json=""
  local -a filters=()
  if [[ -n "$filters_raw" ]]; then
    while IFS= read -r opt; do
      [[ -n "$opt" ]] || continue
      filters+=("$opt")
    done < <(printf '%s\n' "$filters_raw")
  fi
  for opt in "${filters[@]}"; do
    [[ -n "$opt" ]] || continue
    if [[ -n "$filters_json" ]]; then
      filters_json+=","
    fi
    filters_json+="\"$(__dockpipe_sdk_json_escape "$opt")\""
  done
  for opt in "${options[@]}"; do
    if [[ -n "$options_json" ]]; then
      options_json+=","
    fi
    options_json+="\"$(__dockpipe_sdk_json_escape "$opt")\""
  done

  printf '::dockpipe-prompt::{"type":"%s","id":"%s","title":"%s","message":"%s","default":"%s","sensitive":%s,"intent":"%s","automation_group":"%s","allow_auto_approve":%s,"auto_approve_value":"%s","path_mode":"%s","file_filter":"%s","must_exist":%s,"base_dir":"%s","resource_mode":"%s","resource_selection":"%s","resource_kind":"%s","filters":[%s],"options":[%s]}\n' \
    "$(__dockpipe_sdk_json_escape "$prompt_type")" \
    "$(__dockpipe_sdk_json_escape "$prompt_id")" \
    "$(__dockpipe_sdk_json_escape "$title")" \
    "$(__dockpipe_sdk_json_escape "$message")" \
    "$(__dockpipe_sdk_json_escape "$default_value")" \
    "$sensitive" \
    "$(__dockpipe_sdk_json_escape "$intent")" \
    "$(__dockpipe_sdk_json_escape "$automation_group")" \
    "$allow_auto_approve" \
    "$(__dockpipe_sdk_json_escape "$auto_approve_value")" \
    "$(__dockpipe_sdk_json_escape "$path_mode")" \
    "$(__dockpipe_sdk_json_escape "$file_filter")" \
    "$must_exist" \
    "$(__dockpipe_sdk_json_escape "$base_dir")" \
    "$(__dockpipe_sdk_json_escape "$resource_mode")" \
    "$(__dockpipe_sdk_json_escape "$resource_selection")" \
    "$(__dockpipe_sdk_json_escape "$resource_kind")" \
    "$filters_json" \
    "$options_json" >&2
}

__dockpipe_sdk_prompt_read_json_response() {
  local response
  if ! IFS= read -r response; then
    return 1
  fi
  printf '%s\n' "$response"
}

__dockpipe_sdk_json_array() {
  local json="" value
  for value in "$@"; do
    if [[ -n "$json" ]]; then
      json+=","
    fi
    json+="\"$(__dockpipe_sdk_json_escape "$value")\""
  done
  printf '[%s]' "$json"
}

__dockpipe_sdk_join_filters() {
  local joined="" filter
  for filter in "$@"; do
    [[ -n "$filter" ]] || continue
    if [[ -n "$joined" ]]; then
      joined+=";;"
    fi
    joined+="$filter"
  done
  printf '%s' "$joined"
}

__dockpipe_sdk_prompt_confirm_terminal() {
  local title="$1" message="$2" default_value="$3"
  local suffix choice normalized
  case "$default_value" in
    yes|y|true|1) suffix=" [Y/n] " ; default_value="yes" ;;
    no|n|false|0|"") suffix=" [y/N] " ; default_value="no" ;;
    *) suffix=" [y/N] " ; default_value="no" ;;
  esac
  while true; do
    if [[ -n "$title" ]]; then
      printf '%s\n' "$title" >&2
    fi
    printf '%s%s' "$message" "$suffix" >&2
    IFS= read -r choice || return 1
    normalized="$(printf '%s' "${choice:-$default_value}" | tr '[:upper:]' '[:lower:]')"
    case "$normalized" in
      y|yes|true|1) printf 'yes\n'; return 0 ;;
      n|no|false|0) printf 'no\n'; return 0 ;;
    esac
    printf 'Please answer yes or no.\n' >&2
  done
}

__dockpipe_sdk_prompt_input_terminal() {
  local title="$1" message="$2" default_value="$3" sensitive="$4"
  local response
  if [[ -n "$title" ]]; then
    printf '%s\n' "$title" >&2
  fi
  printf '%s' "$message" >&2
  if [[ -n "$default_value" ]]; then
    printf ' [%s]' "$default_value" >&2
  fi
  printf ': ' >&2
  if [[ "$sensitive" == "true" ]]; then
    IFS= read -r -s response || return 1
    printf '\n' >&2
  else
    IFS= read -r response || return 1
  fi
  printf '%s\n' "${response:-$default_value}"
}

__dockpipe_sdk_prompt_resolve_path() {
  local raw="${1:-}"
  [[ -n "$raw" ]] || return 0
  case "$raw" in
    /*)
      printf '%s\n' "$raw"
      return 0
      ;;
    [A-Za-z]:\\*|[A-Za-z]:/*|\\\\*)
      if command -v cygpath >/dev/null 2>&1; then
        cygpath -u "$raw"
      else
        printf '%s\n' "$raw"
      fi
      return 0
      ;;
  esac
  local base="${DOCKPIPE_WORKDIR:-$(pwd)}"
  printf '%s\n' "${base%/}/$raw"
}

__dockpipe_sdk_prompt_trim() {
  local value="${1:-}"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

__dockpipe_sdk_prompt_strip_wrapping_quotes() {
  local value="${1:-}"
  case "$value" in
    \"*\"|\'*\')
      if [[ "${#value}" -ge 2 && "${value:0:1}" == "${value: -1}" ]]; then
        value="${value:1:${#value}-2}"
      fi
      ;;
  esac
  printf '%s' "$value"
}

__dockpipe_sdk_prompt_normalize_path_input() {
  local value
  value="$(__dockpipe_sdk_prompt_trim "${1:-}")"
  value="$(__dockpipe_sdk_prompt_strip_wrapping_quotes "$value")"
  printf '%s\n' "$value"
}

__dockpipe_sdk_prompt_file_terminal() {
  local title="$1" message="$2" default_value="$3" path_mode="$4" must_exist="$5"
  local file_filter="$6"
  local response resolved
  while true; do
    if [[ -n "$title" ]]; then
      printf '%s\n' "$title" >&2
    fi
    printf '%s' "$message" >&2
    if [[ -n "$default_value" ]]; then
      printf ' [%s]' "$default_value" >&2
    fi
    if [[ -n "$file_filter" ]]; then
      printf '\nFilter: %s' "$file_filter" >&2
    fi
    printf ': ' >&2
    IFS= read -r response || return 1
    response="${response:-$default_value}"
    response="$(__dockpipe_sdk_prompt_normalize_path_input "$response")"
    if [[ -z "$response" ]]; then
      printf '%s\n' ""
      return 0
    fi
    if [[ "$must_exist" == "true" ]]; then
      resolved="$(__dockpipe_sdk_prompt_resolve_path "$response")"
      case "$path_mode" in
        open-dir)
          if [[ ! -d "$resolved" ]]; then
            printf 'Directory not found: %s\n' "$response" >&2
            continue
          fi
          ;;
        *)
          if [[ ! -e "$resolved" ]]; then
            printf 'File not found: %s\n' "$response" >&2
            continue
          fi
          ;;
      esac
    fi
    printf '%s\n' "$response"
    return 0
  done
}

__dockpipe_sdk_prompt_parse_resource_entries() {
  local raw="${1:-}"
  local -a entries=()
  local -a parts=()
  local part normalized
  IFS=';' read -r -a parts <<< "$raw"
  for part in "${parts[@]}"; do
    normalized="$(__dockpipe_sdk_prompt_normalize_path_input "$part")"
    [[ -n "$normalized" ]] || continue
    entries+=("$normalized")
  done
  printf '%s\n' "${entries[@]}"
}

__dockpipe_sdk_prompt_validate_resource_entry() {
  local entry="${1:-}" resource_kind="${2:-file}" must_exist="${3:-false}"
  [[ "$must_exist" == "true" ]] || return 0
  local resolved
  resolved="$(__dockpipe_sdk_prompt_resolve_path "$entry")"
  case "$resource_kind" in
    directory)
      if [[ ! -d "$resolved" ]]; then
        printf 'Directory not found: %s\n' "$entry" >&2
        return 1
      fi
      ;;
    *)
      if [[ ! -e "$resolved" ]]; then
        printf 'File not found: %s\n' "$entry" >&2
        return 1
      fi
      ;;
  esac
  return 0
}

__dockpipe_sdk_prompt_resource_terminal() {
  local title="$1" message="$2" default_value="$3" resource_mode="$4" resource_selection="$5" resource_kind="$6" must_exist="$7"
  shift 7 || true
  local -a filters=("$@")
  local filter_text response normalized
  filter_text="$(__dockpipe_sdk_join_filters "${filters[@]}")"
  while true; do
    if [[ -n "$title" ]]; then
      printf '%s\n' "$title" >&2
    fi
    printf '%s' "$message" >&2
    if [[ -n "$default_value" ]]; then
      printf ' [%s]' "$default_value" >&2
    fi
    if [[ -n "$filter_text" ]]; then
      printf '\nFilter: %s' "$filter_text" >&2
    fi
    if [[ "$resource_selection" == "multi" ]]; then
      printf '\nEnter one or more paths separated by ;' >&2
    fi
    printf ': ' >&2
    IFS= read -r response || return 1
    response="${response:-$default_value}"
    if [[ "$resource_selection" == "multi" ]]; then
      local -a entries=()
      local entry
      while IFS= read -r entry; do
        [[ -n "$entry" ]] || continue
        entries+=("$entry")
      done < <(__dockpipe_sdk_prompt_parse_resource_entries "$response")
      if (( ${#entries[@]} == 0 )); then
        printf '%s\n' "${default_value:-[]}"
        return 0
      fi
      local valid="true"
      for entry in "${entries[@]}"; do
        if ! __dockpipe_sdk_prompt_validate_resource_entry "$entry" "$resource_kind" "$must_exist"; then
          valid="false"
          break
        fi
      done
      [[ "$valid" == "true" ]] || continue
      __dockpipe_sdk_json_array "${entries[@]}"
      printf '\n'
      return 0
    fi

    normalized="$(__dockpipe_sdk_prompt_normalize_path_input "$response")"
    if [[ -z "$normalized" ]]; then
      printf '%s\n' ""
      return 0
    fi
    __dockpipe_sdk_prompt_validate_resource_entry "$normalized" "$resource_kind" "$must_exist" || continue
    printf '%s\n' "$normalized"
    return 0
  done
}

__dockpipe_sdk_prompt_choice_terminal() {
  local title="$1" message="$2" default_value="$3"
  shift 3 || true
  local options=("$@")
  local i response idx default_index=1
  if [[ ${#options[@]} -eq 0 ]]; then
    echo "dockpipe sdk: choice prompt requires at least one option" >&2
    return 1
  fi
  if [[ -n "$title" ]]; then
    printf '%s\n' "$title" >&2
  fi
  printf '%s\n' "$message" >&2
  for ((i = 0; i < ${#options[@]}; i++)); do
    if [[ "${options[$i]}" == "$default_value" ]]; then
      default_index=$((i + 1))
    fi
    printf '  %d. %s\n' "$((i + 1))" "${options[$i]}" >&2
  done
  while true; do
    printf 'Choose an option [%d]: ' "$default_index" >&2
    IFS= read -r response || return 1
    idx="${response:-$default_index}"
    if [[ "$idx" =~ ^[0-9]+$ ]] && ((idx >= 1 && idx <= ${#options[@]})); then
      printf '%s\n' "${options[$((idx - 1))]}"
      return 0
    fi
    printf 'Enter a number between 1 and %d.\n' "${#options[@]}" >&2
  done
}

__dockpipe_sdk_prompt() {
  local prompt_type="${1:-}"
  shift || true

  local prompt_id="" title="" message="" default_value="" sensitive="false"
  local intent="" automation_group="" allow_auto_approve="false" auto_approve_value=""
  local path_mode="open-file" file_filter="" must_exist="false"
  local resource_mode="select" resource_selection="single" resource_kind="file"
  local options=()
  local filters=()

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --id)
        prompt_id="${2:-}"
        shift 2 || true
        ;;
      --title)
        title="${2:-}"
        shift 2 || true
        ;;
      --message)
        message="${2:-}"
        shift 2 || true
        ;;
      --default)
        default_value="${2:-}"
        shift 2 || true
        ;;
      --option)
        options+=("${2:-}")
        shift 2 || true
        ;;
      --path-mode)
        path_mode="${2:-}"
        shift 2 || true
        ;;
      --filter)
        file_filter="${2:-}"
        filters+=("${2:-}")
        shift 2 || true
        ;;
      --mode)
        resource_mode="${2:-}"
        shift 2 || true
        ;;
      --selection)
        resource_selection="${2:-}"
        shift 2 || true
        ;;
      --kind)
        resource_kind="${2:-}"
        shift 2 || true
        ;;
      --must-exist)
        must_exist="true"
        shift
        ;;
      --intent)
        intent="${2:-}"
        shift 2 || true
        ;;
      --automation-group)
        automation_group="${2:-}"
        shift 2 || true
        ;;
      --allow-auto-approve)
        allow_auto_approve="true"
        shift
        ;;
      --auto-approve-value)
        auto_approve_value="${2:-}"
        shift 2 || true
        ;;
      --secret|--sensitive)
        sensitive="true"
        shift
        ;;
      *)
        echo "dockpipe sdk: unknown prompt option $1" >&2
        return 1
        ;;
    esac
  done

  if [[ -z "$prompt_id" ]]; then
    prompt_id="prompt.$RANDOM.$RANDOM"
  fi
  if [[ -z "$message" ]]; then
    message="$title"
  fi

  if [[ "$allow_auto_approve" == "true" ]] && __dockpipe_sdk_truthy "${DOCKPIPE_APPROVE_PROMPTS:-}"; then
    if [[ -n "$auto_approve_value" ]]; then
      printf '%s\n' "$auto_approve_value"
      return 0
    fi
    if [[ -n "$default_value" ]]; then
      printf '%s\n' "$default_value"
      return 0
    fi
    case "$prompt_type" in
      confirm)
        printf 'yes\n'
        return 0
        ;;
      choice)
        if [[ ${#options[@]} -gt 0 ]]; then
          printf '%s\n' "${options[0]}"
          return 0
        fi
        ;;
      file|resource)
        if [[ -n "$default_value" ]]; then
          printf '%s\n' "$default_value"
          return 0
        fi
        ;;
    esac
  fi

  case "$(__dockpipe_sdk_prompt_mode)" in
    json)
      if [[ "$prompt_type" == "resource" && ${#filters[@]} -gt 0 ]]; then
        file_filter="$(__dockpipe_sdk_join_filters "${filters[@]}")"
      fi
      local filters_blob=""
      if (( ${#filters[@]} > 0 )); then
        printf -v filters_blob '%s\n' "${filters[@]}"
        filters_blob="${filters_blob%$'\n'}"
      fi
      __dockpipe_sdk_emit_prompt_event "$prompt_type" "$prompt_id" "$title" "$message" "$default_value" "$sensitive" "$intent" "$automation_group" "$allow_auto_approve" "$auto_approve_value" "$path_mode" "$file_filter" "$must_exist" "${DOCKPIPE_WORKDIR:-$(pwd)}" "$resource_mode" "$resource_selection" "$resource_kind" "$filters_blob" "${options[@]}"
      __dockpipe_sdk_prompt_read_json_response
      ;;
    terminal)
      case "$prompt_type" in
        confirm)
          __dockpipe_sdk_prompt_confirm_terminal "$title" "$message" "$default_value"
          ;;
        input)
          __dockpipe_sdk_prompt_input_terminal "$title" "$message" "$default_value" "$sensitive"
          ;;
        choice)
          __dockpipe_sdk_prompt_choice_terminal "$title" "$message" "$default_value" "${options[@]}"
          ;;
        file)
          __dockpipe_sdk_prompt_file_terminal "$title" "$message" "$default_value" "$path_mode" "$must_exist" "$file_filter"
          ;;
        resource)
          __dockpipe_sdk_prompt_resource_terminal "$title" "$message" "$default_value" "$resource_mode" "$resource_selection" "$resource_kind" "$must_exist" "${filters[@]}"
          ;;
        *)
          echo "dockpipe sdk: unknown prompt type $prompt_type" >&2
          return 1
          ;;
      esac
      ;;
    *)
      if [[ -n "$default_value" ]]; then
        printf '%s\n' "$default_value"
        return 0
      fi
      echo "dockpipe sdk: prompt requires a terminal or DOCKPIPE_SDK_PROMPT_MODE=json" >&2
      return 1
      ;;
  esac
}

dockpipe_sdk() {
  local action="${1:-}"
  shift || true
  case "$action" in
    ""|-h|--help|help)
      cat <<'EOF'
dockpipe_sdk actions:
  init-script
  get <workdir|workflow_name|script_dir|package_root|assets_dir|dockpipe_bin|state_dir|artifact_root|output_root|package_id|package_state_dir>
  path <state|build|package|workflow|output|ci> [scope] [suffix...]
  scope [scope|--package name] [suffix...]
  ci <raw|analysis> [suffix...]
  cd-workdir
  die <message...>
  prompt <confirm|choice|input|file|resource> [options]
  require dockpipe-bin
  require workflow-name
  source terraform-pipeline
  refresh [root]
EOF
      ;;
    init-script)
      declare -g ROOT
      declare -g WF_NS
      declare -g SCRIPT_DIR
      SCRIPT_DIR="$(dockpipe_sdk_get script_dir)"
      ROOT="$(dockpipe_sdk_get workdir)"
      if [[ -n "${dockpipe[workflow_name]:-}" ]]; then
        WF_NS="$(dockpipe_sdk_get workflow_name)"
      else
        echo "dockpipe sdk: workflow name is unavailable; init-script requires DOCKPIPE_WORKFLOW_NAME" >&2
        return 1
      fi
      export DOCKPIPE_SCRIPT_DIR="$SCRIPT_DIR"
      export DOCKPIPE_PACKAGE_ROOT="${dockpipe[package_root]:-}"
      export DOCKPIPE_ASSETS_DIR="${dockpipe[assets_dir]:-}"
      cd "$ROOT"
      ;;
    get)
      dockpipe_sdk_get "${1:-}"
      ;;
    scope)
      dockpipe_sdk_scope "$@"
      ;;
    path)
      case "${1:-}" in
        state)
          shift || true
          __dockpipe_sdk_join_path "$(dockpipe_sdk_state_dir)" "$@"
          ;;
        build)
          shift || true
          dockpipe_sdk_build_dir "$@"
          ;;
        package)
          shift || true
          dockpipe_sdk_package_state_dir "$@"
          ;;
        workflow)
          shift || true
          dockpipe_sdk_workflow_state_dir "$@"
          ;;
        output)
          shift || true
          __dockpipe_sdk_join_path "$(dockpipe_sdk_get output_root)" "$@"
          ;;
        ci)
          shift || true
          local kind="${1:-}"
          shift || true
          __dockpipe_sdk_join_path "$(dockpipe_sdk_ci_artifact_dir "$kind")" "$@"
          ;;
        *)
          echo "dockpipe sdk: unknown path target ${1:-<empty>}" >&2
          return 1
          ;;
      esac
      ;;
    ci)
      local kind="${1:-}"
      shift || true
      __dockpipe_sdk_join_path "$(dockpipe_sdk_ci_artifact_dir "$kind")" "$@"
      ;;
    bind)
      case "${1:-}" in
        ci-artifacts)
          shift || true
          dockpipe_sdk_bind_ci_artifacts "${1:-}"
          ;;
        *)
          echo "dockpipe sdk: unknown bind target ${1:-<empty>}" >&2
          return 1
          ;;
      esac
      ;;
    cd-workdir)
      cd "$(dockpipe_sdk_get workdir)"
      ;;
    die)
      __dockpipe_sdk_fatal "$@"
      ;;
    prompt)
      __dockpipe_sdk_prompt "$@"
      ;;
    require)
      case "${1:-}" in
        dockpipe-bin)
          dockpipe_require_dockpipe_bin
          ;;
        workflow-name)
          if [[ -n "${dockpipe[workflow_name]:-}" ]]; then
            printf '%s\n' "${dockpipe[workflow_name]}"
          else
            echo "dockpipe sdk: workflow name is unavailable; this action requires DOCKPIPE_WORKFLOW_NAME" >&2
            return 1
          fi
          ;;
        *)
          echo "dockpipe sdk: unknown require target ${1:-<empty>}" >&2
          return 1
          ;;
      esac
      ;;
    source)
      case "${1:-}" in
        terraform-pipeline)
          local dockpipe_bin pipeline_sh
          dockpipe_bin="$(dockpipe_require_dockpipe_bin)" || return 1
          pipeline_sh="$("$dockpipe_bin" terraform pipeline-path 2>/dev/null)" || {
            echo "dockpipe sdk: could not resolve terraform pipeline path via dockpipe terraform pipeline-path" >&2
            return 1
          }
          [[ -f "$pipeline_sh" ]] || {
            echo "dockpipe sdk: terraform pipeline script not found at ${pipeline_sh:-<empty>}" >&2
            return 1
          }
          # shellcheck source=/dev/null
          source "$pipeline_sh"
          ;;
        *)
          echo "dockpipe sdk: unknown source target ${1:-<empty>}" >&2
          return 1
          ;;
      esac
      ;;
    refresh)
      dockpipe_sdk_refresh "${1:-}"
      ;;
    *)
      echo "dockpipe sdk: unknown action $action" >&2
      return 1
      ;;
  esac
}

dockpipe_sdk_refresh "${DOCKPIPE_WORKDIR:-}"
