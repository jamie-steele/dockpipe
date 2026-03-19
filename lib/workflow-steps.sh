#!/usr/bin/env bash
# Multi-step workflows: parse steps from config, run each with run → isolate → act,
# merge .dockpipe/outputs.env (or per-step outputs path) into env for the next step.
# Sourced from bin/dockpipe after resolve_template is defined; uses dockpipe_run from runner.sh.

set -euo pipefail

dockpipe_config_has_steps() {
  local f="${1:?}"
  [[ -f "$f" ]] && grep -qE '^steps:[[:space:]]*(#|$)' "$f"
}

dockpipe_var_is_locked() {
  local key="$1"
  local k
  for k in "${DOCKPIPE_LOCKED_VAR_NAMES[@]+"${DOCKPIPE_LOCKED_VAR_NAMES[@]}"}"; do
    [[ "$k" == "$key" ]] && return 0
  done
  return 1
}

# Apply KEY=VAL lines: export for host (pre-scripts) and append to DOCKPIPE_EXTRA_ENV for the next container.
# Keys locked by --var are skipped.
dockpipe_merge_outputs_env_file() {
  local file="$1"
  [[ -f "$file" ]] || return 0
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    [[ -z "${line// }" ]] && continue
    [[ "$line" != *=* ]] && continue
    local key="${line%%=*}"
    key="${key%"${key##*[![:space:]]}"}"
    key="${key#"${key%%[![:space:]]*}"}"
    [[ -z "$key" ]] && continue
    dockpipe_var_is_locked "$key" && continue
    local val="${line#*=}"
    val="${val#"${val%%[![:space:]]*}"}"
    val="${val%"${val##*[![:space:]]}"}"
    if [[ "${val:0:1}" == '"' ]] && [[ "${val: -1}" == '"' ]]; then
      val="${val:1:-1}"
    elif [[ "${val:0:1}" == "'" ]] && [[ "${val: -1}" == "'" ]]; then
      val="${val:1:-1}"
    fi
    export "${key}=${val}"
    DOCKPIPE_EXTRA_ENV="${DOCKPIPE_EXTRA_ENV:+$DOCKPIPE_EXTRA_ENV$'\n'}${key}=${val}"
  done < "$file"
}

dockpipe_absorb_step_outputs() {
  local workdir="${1:?}"
  local rel="${2:-.dockpipe/outputs.env}"
  rel="${rel#/}"
  local path="${workdir}/${rel}"
  [[ -f "$path" ]] || return 0
  echo "[dockpipe] Merging outputs from ${rel} into environment (next step)" >&2
  dockpipe_merge_outputs_env_file "$path"
  rm -f "$path"
}

# Resolve paths for run/act like bin/dockpipe (repo vs workflow-relative).
dockpipe_step_resolve_script_path() {
  local rel="$1"
  local workflow_root="$2"
  local repo_root="$3"
  [[ -z "$rel" ]] && return
  if [[ "$rel" == scripts/* ]]; then
    echo "${repo_root}/${rel}"
  else
    echo "${workflow_root}/${rel}"
  fi
}

dockpipe_workflow_run_steps() {
  local config_file="${1:?}"
  local workflow_root="${2:?}"
  local repo_root="${3:?}"
  shift 3
  local -a cli_cmd=("$@")

  local py="${repo_root}/lib/parse_workflow_steps.py"
  if [[ ! -f "$py" ]]; then
    echo "Error: missing ${py}" >&2
    return 1
  fi
  if ! command -v python3 &>/dev/null; then
    echo "Error: python3 is required for workflows with steps: in config.yml" >&2
    return 1
  fi

  local staging
  staging="$(mktemp -d "${TMPDIR:-/tmp}/dockpipe-steps.XXXXXX")"
  trap 'rm -rf "${staging}"' RETURN

  python3 "$py" "$config_file" "$staging" || {
    echo "Error: failed to parse steps: in ${config_file}" >&2
    return 1
  }

  local n
  n="$(cat "${staging}/n_steps")"
  [[ "$n" =~ ^[0-9]+$ ]] || {
    echo "Error: invalid step count" >&2
    return 1
  }
  [[ "$n" -ge 1 ]] || {
    echo "Error: no steps defined" >&2
    return 1
  }

  local default_isolate default_act
  default_isolate="$(grep -E "^isolate:" "$config_file" 2>/dev/null | sed -E 's/^[^:]+:[[:space:]]*//' | head -1 || true)"
  default_act="$(grep -E "^act:" "$config_file" 2>/dev/null | sed -E 's/^[^:]+:[[:space:]]*//' | head -1 || true)"
  [[ -z "$default_act" ]] && default_act="$(grep -E "^action:" "$config_file" 2>/dev/null | sed -E 's/^[^:]+:[[:space:]]*//' | head -1 || true)"

  local idx
  for ((idx = 0; idx < n; idx++)); do
    local sdir="${staging}/step_${idx}"
    local skip_c act_raw iso_raw out_rel
    skip_c="$(cat "${sdir}/skip_container")"
    act_raw="$(cat "${sdir}/act")"
    out_rel="$(cat "${sdir}/outputs")"
    iso_raw="$(cat "${sdir}/isolate")"
    [[ -z "${out_rel// }" ]] && out_rel=".dockpipe/outputs.env"

    echo "[dockpipe] --- Step $((idx + 1))/${n} ---" >&2

    # Step-local vars (override prior step outputs for keys listed here; respect locks)
    if [[ -f "${sdir}/vars.env" ]]; then
      dockpipe_merge_outputs_env_file "${sdir}/vars.env"
    fi

    # Host run scripts for this step
    local -a pre_list=()
    while IFS= read -r rline || [[ -n "$rline" ]]; do
      [[ -z "${rline// }" ]] && continue
      pre_list+=("$(dockpipe_step_resolve_script_path "$rline" "$workflow_root" "$repo_root")")
    done < "${sdir}/run"

    if [[ "$idx" -eq 0 ]]; then
      local extra
      for extra in "${DOCKPIPE_FIRST_STEP_EXTRA_PRE[@]+"${DOCKPIPE_FIRST_STEP_EXTRA_PRE[@]}"}"; do
        [[ -z "$extra" ]] && continue
        local ep="$extra"
        if [[ "$ep" != /* ]]; then
          if [[ -f "${repo_root}/${ep}" ]]; then
            ep="${repo_root}/${ep}"
          fi
        fi
        pre_list+=("$ep")
      done
    fi

    for _pre_path in "${pre_list[@]+"${pre_list[@]}"}"; do
      [[ -z "${_pre_path:-}" ]] && continue
      if [[ ! -f "$_pre_path" ]]; then
        echo "Error: pre-script not found: ${_pre_path}" >&2
        return 1
      fi
      echo "[dockpipe] Running pre-script: ${_pre_path}" >&2
      # shellcheck source=/dev/null
      source "${_pre_path}"
    done
    unset _pre_path

    if [[ -n "${DOCKPIPE_COMMIT_ON_HOST:-}" ]] && [[ -n "${DOCKPIPE_EXTRA_ENV:-}" ]]; then
      while IFS= read -r e; do
        case "$e" in
          DOCKPIPE_COMMIT_MESSAGE=*) export "$e" ;;
          DOCKPIPE_WORK_BRANCH=*) export "$e" ;;
          DOCKPIPE_BUNDLE_OUT=*) export "$e" ;;
          DOCKPIPE_BUNDLE_ALL=*) export "$e" ;;
          GIT_PAT=*) export "$e" ;;
        esac
      done <<< "${DOCKPIPE_EXTRA_ENV}"
    fi

    # argv for container
    local -a step_argv=()
    if [[ -s "${sdir}/argv" ]]; then
      while IFS= read -r -d '' arg || [[ -n "${arg:-}" ]]; do
        step_argv+=("$arg")
      done < "${sdir}/argv"
    fi

    if [[ "$idx" -eq $((n - 1)) ]] && [[ ${#step_argv[@]} -eq 0 ]] && [[ ${#cli_cmd[@]} -gt 0 ]]; then
      step_argv=("${cli_cmd[@]}")
    fi

    if [[ "$skip_c" == "1" ]]; then
      dockpipe_absorb_step_outputs "${DOCKPIPE_WORKDIR:-$(pwd)}" "$out_rel"
      continue
    fi

    if [[ ${#step_argv[@]} -eq 0 ]]; then
      echo "Error: step $((idx + 1)) has no cmd/command and no command was passed after --" >&2
      return 1
    fi

    # Effective isolate / act (step → CLI override → workflow default)
    local eff_iso eff_act
    eff_iso="${iso_raw}"
    [[ -z "${eff_iso// }" ]] && eff_iso="${DOCKPIPE_USER_ISOLATE_OVERRIDE:-}"
    [[ -z "${eff_iso// }" ]] && eff_iso="${default_isolate:-}"
    [[ -z "${eff_iso// }" ]] && eff_iso="${RESOLVER:-}"

    eff_act="${act_raw}"
    [[ -z "${eff_act// }" ]] && eff_act="${DOCKPIPE_USER_ACT_OVERRIDE:-}"
    [[ -z "${eff_act// }" ]] && eff_act="${default_act:-}"

    DOCKPIPE_ISOLATE="${eff_iso}"
    TEMPLATE=""
    DOCKPIPE_IMAGE=""
    local _iso_resolved
    _iso_resolved=$(resolve_template "${DOCKPIPE_ISOLATE}")
    if [[ -n "$_iso_resolved" ]]; then
      TEMPLATE="${DOCKPIPE_ISOLATE}"
    else
      DOCKPIPE_IMAGE="${DOCKPIPE_ISOLATE}"
    fi
    unset _iso_resolved

    if [[ -n "${TEMPLATE}" ]]; then
      local resolved
      resolved=$(resolve_template "$TEMPLATE")
      if [[ -z "$resolved" ]]; then
        echo "Error: unknown template '${TEMPLATE}'" >&2
        return 1
      fi
      DOCKPIPE_IMAGE="${resolved%% *}"
      local build_path="${resolved#* }"
      if [[ -d "$build_path" ]]; then
        DOCKPIPE_BUILD="$build_path"
        DOCKPIPE_BUILD_CONTEXT="${repo_root}"
      else
        unset DOCKPIPE_BUILD DOCKPIPE_BUILD_CONTEXT
      fi
    else
      unset DOCKPIPE_BUILD DOCKPIPE_BUILD_CONTEXT
    fi

    if [[ -z "${DOCKPIPE_IMAGE}" ]]; then
      DOCKPIPE_IMAGE="dockpipe-base-dev"
      DOCKPIPE_BUILD="${repo_root}/images/base-dev"
      DOCKPIPE_BUILD_CONTEXT="${repo_root}"
    fi

    # Only tag built-in dockpipe images; do not append repo version to Docker Hub names (e.g. alpine).
    if [[ "${DOCKPIPE_IMAGE}" != *:* ]] && [[ -f "${repo_root}/version" ]]; then
      case "${DOCKPIPE_IMAGE}" in
        dockpipe-*) DOCKPIPE_IMAGE="${DOCKPIPE_IMAGE}:$(cat "${repo_root}/version")" ;;
      esac
    fi

    DOCKPIPE_ACTION=""
    if [[ -n "${eff_act// }" ]]; then
      DOCKPIPE_ACTION="$(dockpipe_step_resolve_script_path "$eff_act" "$workflow_root" "$repo_root")"
    fi

    if [[ -n "${DOCKPIPE_ACTION}" ]] && [[ "${DOCKPIPE_ACTION}" != /* ]]; then
      if [[ -f "${repo_root}/${DOCKPIPE_ACTION}" ]]; then
        DOCKPIPE_ACTION="${repo_root}/${DOCKPIPE_ACTION}"
      elif [[ -f "${repo_root}/scripts/${DOCKPIPE_ACTION}" ]]; then
        DOCKPIPE_ACTION="${repo_root}/scripts/${DOCKPIPE_ACTION}"
      elif [[ -f "${DOCKPIPE_ACTION}" ]]; then
        DOCKPIPE_ACTION="$(cd "$(dirname "${DOCKPIPE_ACTION}")" && pwd)/$(basename "${DOCKPIPE_ACTION}")"
      else
        DOCKPIPE_ACTION="$(pwd)/${DOCKPIPE_ACTION}"
      fi
    fi
    if [[ -n "${DOCKPIPE_ACTION}" ]] && [[ ! -f "${DOCKPIPE_ACTION}" ]]; then
      echo "Error: action script not found: ${DOCKPIPE_ACTION}" >&2
      return 1
    fi

    local bundled_commits=(
      "${repo_root}/scripts/commit-worktree.sh"
    )
    unset DOCKPIPE_COMMIT_ON_HOST
    if [[ -n "${DOCKPIPE_ACTION}" ]]; then
      local resolved_action resolved_bundled is_bundled_commit=""
      resolved_action="$(cd "$(dirname "${DOCKPIPE_ACTION}")" && pwd)/$(basename "${DOCKPIPE_ACTION}")"
      local bundled_commit
      for bundled_commit in "${bundled_commits[@]}"; do
        if [[ -f "${bundled_commit}" ]]; then
          resolved_bundled="$(cd "$(dirname "${bundled_commit}")" && pwd)/$(basename "${bundled_commit}")"
          if [[ "$resolved_action" == "$resolved_bundled" ]]; then
            is_bundled_commit=1
            break
          fi
        fi
      done
      if [[ -n "${is_bundled_commit}" ]]; then
        DOCKPIPE_COMMIT_ON_HOST=1
        export DOCKPIPE_COMMIT_ON_HOST
        if [[ -z "${DOCKPIPE_BRANCH_PREFIX:-}" ]]; then
          if [[ -n "${RESOLVER:-}" ]]; then
            export DOCKPIPE_BRANCH_PREFIX="${RESOLVER}"
          elif [[ -n "${TEMPLATE:-}" ]]; then
            case "$TEMPLATE" in
              claude|agent-dev) export DOCKPIPE_BRANCH_PREFIX=claude ;;
              codex) export DOCKPIPE_BRANCH_PREFIX=codex ;;
              *) export DOCKPIPE_BRANCH_PREFIX=dockpipe ;;
            esac
          fi
        fi
        while IFS= read -r e; do
          case "$e" in
            DOCKPIPE_COMMIT_MESSAGE=*) export "$e" ;;
            DOCKPIPE_WORK_BRANCH=*) export "$e" ;;
            DOCKPIPE_BUNDLE_OUT=*) export "$e" ;;
            DOCKPIPE_BUNDLE_ALL=*) export "$e" ;;
          esac
        done <<< "${DOCKPIPE_EXTRA_ENV:-}"
        DOCKPIPE_ACTION=""
      fi
    fi

    export DOCKPIPE_IMAGE DOCKPIPE_ACTION TEMPLATE

    if [[ -n "${DOCKPIPE_BUILD:-}" ]] && [[ -n "${DOCKPIPE_BUILD_CONTEXT:-}" ]]; then
      export DOCKPIPE_BUILD DOCKPIPE_BUILD_CONTEXT
      local img_base="${DOCKPIPE_IMAGE%%:*}"
      if [[ "$img_base" == "dockpipe-dev" ]]; then
        if ! docker image inspect dockpipe-base-dev:latest &>/dev/null; then
          docker build -q -t dockpipe-base-dev -f "${repo_root}/images/base-dev/Dockerfile" "${DOCKPIPE_BUILD_CONTEXT}"
        fi
      fi
      docker build -q -t "${DOCKPIPE_IMAGE}" -f "${DOCKPIPE_BUILD}/Dockerfile" "${DOCKPIPE_BUILD_CONTEXT}"
      unset DOCKPIPE_BUILD DOCKPIPE_BUILD_CONTEXT
    fi

    local rc=0
    dockpipe_run "${step_argv[@]}" || rc=$?

    if [[ $rc -ne 0 ]]; then
      echo "[dockpipe] Step $((idx + 1)) failed with exit code ${rc}" >&2
      return "$rc"
    fi

    dockpipe_absorb_step_outputs "${DOCKPIPE_WORKDIR:-$(pwd)}" "$out_rel"
  done

  return 0
}
