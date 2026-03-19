#!/usr/bin/env bash
# Workflow variables: defaults from config.yml vars:, then .env files, then --var.
# Precedence (low → high): vars in yml < template/.env < repo .env < extra --env-file(s) < DOCKPIPE_ENV_FILE < --var
# Already-exported environment variables are never overwritten by yml or .env; use --var to force.

# dockpipe_dotenv_merge_if_unset <file>
# For each KEY=VAL line, export only if KEY is not already set (bash -v).
dockpipe_dotenv_merge_if_unset() {
  local f="$1"
  [[ -f "$f" ]] || return 0
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    [[ -z "${line// }" ]] && continue
    [[ "$line" != *=* ]] && continue
    local key="${line%%=*}"
    key="${key%"${key##*[![:space:]]}"}"
    key="${key#"${key%%[![:space:]]*}"}"
    [[ -z "$key" ]] || [[ "$key" == \#* ]] && continue
    local val="${line#*=}"
    val="${val#"${val%%[![:space:]]*}"}"
    val="${val%"${val##*[![:space:]]}"}"
    if [[ "${val:0:1}" == '"' ]] && [[ "${val: -1}" == '"' ]]; then
      val="${val:1:-1}"
    elif [[ "${val:0:1}" == "'" ]] && [[ "${val: -1}" == "'" ]]; then
      val="${val:1:-1}"
    fi
    if [[ ! -v "$key" ]]; then
      export "${key}=${val}"
    fi
  done < "$f"
}

# dockpipe_yaml_vars_apply_defaults <config.yml>
# Under top-level `vars:`, each `  KEY: value` line sets KEY if unset. Stops at next top-level key.
dockpipe_yaml_vars_apply_defaults() {
  local cfg="$1"
  [[ -f "$cfg" ]] || return 0
  local in_vars=0
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" =~ ^vars:[[:space:]]* ]] || [[ "$line" =~ ^vars:[[:space:]]*# ]]; then
      in_vars=1
      continue
    fi
    if [[ "$in_vars" -eq 1 ]]; then
      if [[ "$line" =~ ^[[:space:]] ]] || [[ -z "${line// }" ]] || [[ "$line" =~ ^[[:space:]]*# ]]; then
        :
      elif [[ "$line" =~ ^[a-zA-Z_][a-zA-Z0-9_-]*: ]]; then
        in_vars=0
        continue
      else
        in_vars=0
        continue
      fi
    fi
    [[ "$in_vars" -eq 0 ]] && continue
    [[ "$line" =~ ^[[:space:]]+([a-zA-Z_][a-zA-Z0-9_]*):[[:space:]]*(.*)$ ]] || continue
    local _k="${BASH_REMATCH[1]}"
    local _v="${BASH_REMATCH[2]}"
    _v="${_v#"${_v%%[![:space:]]*}"}"
    _v="${_v%"${_v##*[![:space:]]}"}"
    _v="${_v%%#*}"
    _v="${_v%"${_v##*[![:space:]]}"}"
    if [[ "${_v:0:1}" == '"' ]] && [[ "${_v: -1}" == '"' ]]; then
      _v="${_v:1:-1}"
    elif [[ "${_v:0:1}" == "'" ]] && [[ "${_v: -1}" == "'" ]]; then
      _v="${_v:1:-1}"
    fi
    if [[ ! -v "$_k" ]]; then
      export "${_k}=${_v}"
    fi
  done < "$cfg"
}

# dockpipe_workflow_apply_vars <config.yml> <workflow_root> <repo_root>
# Optional env: DOCKPIPE_ENV_FILE — extra .env path
# Uses global arrays: DOCKPIPE_ENV_FILE_EXTRAS (from --env-file), DOCKPIPE_WORKFLOW_VAR_OVERRIDES (key=value from --var)
dockpipe_workflow_apply_vars() {
  local cfg="$1"
  local wf_root="$2"
  local repo_root="$3"
  dockpipe_yaml_vars_apply_defaults "$cfg"
  dockpipe_dotenv_merge_if_unset "${wf_root}/.env"
  dockpipe_dotenv_merge_if_unset "${repo_root}/.env"
  local ef
  for ef in "${DOCKPIPE_ENV_FILE_EXTRAS[@]+"${DOCKPIPE_ENV_FILE_EXTRAS[@]}"}"; do
    [[ -n "$ef" ]] && dockpipe_dotenv_merge_if_unset "$ef"
  done
  [[ -n "${DOCKPIPE_ENV_FILE:-}" ]] && dockpipe_dotenv_merge_if_unset "${DOCKPIPE_ENV_FILE}"
  local pair
  for pair in "${DOCKPIPE_WORKFLOW_VAR_OVERRIDES[@]+"${DOCKPIPE_WORKFLOW_VAR_OVERRIDES[@]}"}"; do
    [[ -z "$pair" ]] && continue
    [[ "$pair" != *=* ]] && continue
    export "${pair%%=*}=${pair#*=}"
  done
}
