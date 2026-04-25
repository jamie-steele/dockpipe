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
