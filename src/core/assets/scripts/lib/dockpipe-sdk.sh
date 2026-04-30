#!/usr/bin/env bash

dockpipe_repo_root() {
  local root="${1:-}"
  if [[ -z "$root" ]]; then
    root="${DOCKPIPE_WORKDIR:-$(pwd)}"
  fi
  (cd "$root" && pwd)
}

dockpipe_resolve_dockpipe_bin() {
  local root
  root="$(dockpipe_repo_root "${1:-}")"
  if [[ -n "${DOCKPIPE_BIN:-}" ]]; then
    printf '%s\n' "$DOCKPIPE_BIN"
    return 0
  fi
  local candidate="$root/src/bin/dockpipe"
  if [[ -x "$candidate" ]]; then
    printf '%s\n' "$candidate"
    return 0
  fi
  command -v dockpipe 2>/dev/null || return 1
}

dockpipe_sdk_refresh() {
  local root="${1:-}"
  local resolved_root resolved_dockpipe resolved_workflow_name resolved_script_dir resolved_assets_dir resolved_package_root
  resolved_root="$(dockpipe_repo_root "$root")"
  resolved_dockpipe="$(dockpipe_resolve_dockpipe_bin "$resolved_root" 2>/dev/null || true)"
  resolved_workflow_name="${DOCKPIPE_WORKFLOW_NAME:-}"
  resolved_script_dir="$(dockpipe_script_dir)"
  resolved_assets_dir="$(dockpipe_find_assets_dir "$resolved_script_dir")"
  resolved_package_root="$(dockpipe_find_package_root "$resolved_script_dir")"

  declare -gA dockpipe
  dockpipe=()
  dockpipe[workdir]="$resolved_root"
  dockpipe[dockpipe_bin]="$resolved_dockpipe"
  dockpipe[workflow_name]="$resolved_workflow_name"
  dockpipe[script_dir]="$resolved_script_dir"
  dockpipe[assets_dir]="$resolved_assets_dir"
  dockpipe[package_root]="$resolved_package_root"

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
    workdir|dockpipe_bin|workflow_name|script_dir|package_root|assets_dir)
      printf '%s\n' "${dockpipe[$field]:-}"
      ;;
    *)
      echo "dockpipe sdk: unknown field ${field:-<empty>}" >&2
      return 1
      ;;
  esac
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
  if [[ "${DOCKPIPE_SDK_PROMPT_MODE:-}" == "json" ]]; then
    printf 'json\n'
    return 0
  fi
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
  shift 10 || true
  local options=("$@")
  local options_json="" opt
  for opt in "${options[@]}"; do
    if [[ -n "$options_json" ]]; then
      options_json+=","
    fi
    options_json+="\"$(__dockpipe_sdk_json_escape "$opt")\""
  done

  printf '::dockpipe-prompt::{"type":"%s","id":"%s","title":"%s","message":"%s","default":"%s","sensitive":%s,"intent":"%s","automation_group":"%s","allow_auto_approve":%s,"auto_approve_value":"%s","options":[%s]}\n' \
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
    "$options_json" >&2
}

__dockpipe_sdk_prompt_read_json_response() {
  local response
  if ! IFS= read -r response; then
    return 1
  fi
  printf '%s\n' "$response"
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
  local options=()

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
    esac
  fi

  case "$(__dockpipe_sdk_prompt_mode)" in
    json)
      __dockpipe_sdk_emit_prompt_event "$prompt_type" "$prompt_id" "$title" "$message" "$default_value" "$sensitive" "$intent" "$automation_group" "$allow_auto_approve" "$auto_approve_value" "${options[@]}"
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
  get <workdir|workflow_name|script_dir|package_root|assets_dir|dockpipe_bin>
  cd-workdir
  die <message...>
  prompt <confirm|choice|input> [options]
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
