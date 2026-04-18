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

dockpipe_resolve_dorkpipe_bin() {
  local root
  root="$(dockpipe_repo_root "${1:-}")"
  if [[ -n "${DORKPIPE_BIN:-}" ]]; then
    printf '%s\n' "$DORKPIPE_BIN"
    return 0
  fi
  local candidate="$root/packages/dorkpipe/bin/dorkpipe"
  if [[ -x "$candidate" ]]; then
    printf '%s\n' "$candidate"
    return 0
  fi
  command -v dorkpipe 2>/dev/null || return 1
}

dockpipe_sdk_refresh() {
  local root="${1:-}"
  local resolved_root resolved_dockpipe resolved_dorkpipe resolved_workflow_name
  resolved_root="$(dockpipe_repo_root "$root")"
  resolved_dockpipe="$(dockpipe_resolve_dockpipe_bin "$resolved_root" 2>/dev/null || true)"
  resolved_dorkpipe="$(dockpipe_resolve_dorkpipe_bin "$resolved_root" 2>/dev/null || true)"
  resolved_workflow_name="${DOCKPIPE_WORKFLOW_NAME:-}"

  declare -gA dockpipe
  dockpipe=()
  dockpipe[workdir]="$resolved_root"
  dockpipe[dockpipe_bin]="$resolved_dockpipe"
  dockpipe[dorkpipe_bin]="$resolved_dorkpipe"
  dockpipe[workflow_name]="$resolved_workflow_name"

  export DOCKPIPE_SDK_ROOT="$resolved_root"
  if [[ -n "$resolved_dockpipe" ]]; then
    export DOCKPIPE_BIN="$resolved_dockpipe"
  fi
  if [[ -n "$resolved_dorkpipe" ]]; then
    export DORKPIPE_BIN="$resolved_dorkpipe"
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

dockpipe_require_dorkpipe_bin() {
  local bin="${DORKPIPE_BIN:-${dockpipe[dorkpipe_bin]:-}}"
  if [[ -z "$bin" ]]; then
    echo "dockpipe sdk: dorkpipe binary not found; set DORKPIPE_BIN or add dorkpipe to PATH" >&2
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
  script-dir
  workdir
  cd-workdir
  die <message...>
  workflow-name
  require dockpipe-bin
  require dorkpipe-bin
  require workflow-name
  source terraform-pipeline
  refresh [root]
EOF
      ;;
    init-script)
      declare -g ROOT
      declare -g WF_NS
      declare -g SCRIPT_DIR
      SCRIPT_DIR="$(dockpipe_script_dir)"
      ROOT="$(printf '%s\n' "${dockpipe[workdir]}")"
      if [[ -n "${dockpipe[workflow_name]:-}" ]]; then
        WF_NS="${dockpipe[workflow_name]}"
      else
        echo "dockpipe sdk: workflow name is unavailable; init-script requires DOCKPIPE_WORKFLOW_NAME" >&2
        return 1
      fi
      cd "$ROOT"
      ;;
    script-dir)
      dockpipe_script_dir
      ;;
    workdir)
      printf '%s\n' "${dockpipe[workdir]}"
      ;;
    cd-workdir)
      cd "${dockpipe[workdir]}"
      ;;
    die)
      __dockpipe_sdk_fatal "$@"
      ;;
    workflow-name)
      dockpipe_workflow_name
      ;;
    require)
      case "${1:-}" in
        dockpipe-bin)
          dockpipe_require_dockpipe_bin
          ;;
        dorkpipe-bin)
          dockpipe_require_dorkpipe_bin
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
