#!/usr/bin/env bash
# shellcheck shell=bash
# Reusable Terraform command pipeline for DockPipe host workflows (source this file).
#
# Convention: environment variables use the DOCKPIPE_TF_* namespace. Optional compatibility
# mappers (e.g. dockpipe_tf_map_r2_publish_env) set DOCKPIPE_TF_* from workflow-specific names.
#
# Typical use:
#   source "$ROOT/templates/core/assets/scripts/terraform-pipeline.sh"  # or src/core/... in repo
#   export DOCKPIPE_TF_LOG_PREFIX=my-workflow
#   dockpipe_tf_map_r2_publish_env   # if using R2_* names from dockpipe.cloudflare.r2publish (r2-publish)
#   export TF_VAR_...
#   dockpipe_tf_run_pipeline "$tf_dir" "$path/to/backend.hcl"
#
# See: src/core/assets/scripts/README.md (terraform-pipeline section)

[[ -n "${BASH_VERSION:-}" ]] || {
  echo "terraform-pipeline.sh: bash required" >&2
  return 1 2>/dev/null || exit 1
}

dockpipe_tf_log() {
  echo "${DOCKPIPE_TF_LOG_PREFIX:-dockpipe-tf}: $*"
}

dockpipe_tf_die() {
  echo "${DOCKPIPE_TF_LOG_PREFIX:-dockpipe-tf}: $*" >&2
  exit 1
}

# Map legacy R2_TERRAFORM_* / R2_TF_* into DOCKPIPE_TF_* when unset (Cloudflare R2 publish workflow).
dockpipe_tf_map_r2_publish_env() {
  export DOCKPIPE_TF_COMMANDS="${DOCKPIPE_TF_COMMANDS:-${R2_TERRAFORM_COMMANDS:-init,apply}}"
  export DOCKPIPE_TF_SKIP_INIT="${DOCKPIPE_TF_SKIP_INIT:-${R2_TERRAFORM_SKIP_INIT:-0}}"
  export DOCKPIPE_TF_INIT_ARGS="${DOCKPIPE_TF_INIT_ARGS:-${R2_TERRAFORM_INIT_ARGS:-}}"
  export DOCKPIPE_TF_PLAN_ARGS="${DOCKPIPE_TF_PLAN_ARGS:-${R2_TERRAFORM_PLAN_ARGS:-}}"
  export DOCKPIPE_TF_APPLY_ARGS="${DOCKPIPE_TF_APPLY_ARGS:-${R2_TERRAFORM_APPLY_ARGS:-}}"
  export DOCKPIPE_TF_APPLY_AUTO_APPROVE="${DOCKPIPE_TF_APPLY_AUTO_APPROVE:-${R2_TERRAFORM_APPLY_AUTO_APPROVE:-1}}"
  export DOCKPIPE_TF_VALIDATE_ARGS="${DOCKPIPE_TF_VALIDATE_ARGS:-${R2_TERRAFORM_VALIDATE_ARGS:-}}"
  export DOCKPIPE_TF_FMT_ARGS="${DOCKPIPE_TF_FMT_ARGS:-${R2_TERRAFORM_FMT_ARGS:-}}"
  export DOCKPIPE_TF_IMPORT_ARGS="${DOCKPIPE_TF_IMPORT_ARGS:-${R2_TERRAFORM_IMPORT_ARGS:-}}"
  export DOCKPIPE_TF_IMPORT_FILE="${DOCKPIPE_TF_IMPORT_FILE:-${R2_TERRAFORM_IMPORT_FILE:-}}"
  export DOCKPIPE_TF_BACKEND="${DOCKPIPE_TF_BACKEND:-${R2_TF_BACKEND:-remote}}"
  export DOCKPIPE_TF_STATE_BUCKET="${DOCKPIPE_TF_STATE_BUCKET:-${R2_TF_STATE_BUCKET:-dockpipe}}"
  export DOCKPIPE_TF_STATE_KEY="${DOCKPIPE_TF_STATE_KEY:-${R2_TF_STATE_KEY:-state/dockpipe.cloudflare.r2publish/terraform.tfstate}}"
  export DOCKPIPE_TF_STATE_ACCESS_KEY_ID="${DOCKPIPE_TF_STATE_ACCESS_KEY_ID:-${R2_STATE_ACCESS_KEY_ID:-}}"
  export DOCKPIPE_TF_STATE_SECRET_ACCESS_KEY="${DOCKPIPE_TF_STATE_SECRET_ACCESS_KEY:-${R2_STATE_SECRET_ACCESS_KEY:-}}"
  export DOCKPIPE_TF_DRY_RUN="${DOCKPIPE_TF_DRY_RUN:-${R2_PUBLISH_DRY_RUN:-0}}"
}

# Write Terraform s3 backend config for Cloudflare R2 (same shape as scripts/dockpipe/r2-publish.sh).
dockpipe_tf_write_r2_remote_backend_config() {
  local out="$1"
  local account="${DOCKPIPE_TF_CLOUDFLARE_ACCOUNT_ID:-${CLOUDFLARE_ACCOUNT_ID:-}}"
  [[ -n "$account" ]] || dockpipe_tf_die "CLOUDFLARE_ACCOUNT_ID or DOCKPIPE_TF_CLOUDFLARE_ACCOUNT_ID required for remote R2 backend"
  local bucket="${DOCKPIPE_TF_STATE_BUCKET:-dockpipe}"
  local key="${DOCKPIPE_TF_STATE_KEY:-state/terraform.tfstate}"
  local ep="https://${account}.r2.cloudflarestorage.com"
  cat >"$out" <<EOF
bucket = "${bucket}"
key    = "${key}"
region = "auto"
skip_credentials_validation = true
skip_metadata_api_check     = true
skip_region_validation      = true
skip_requesting_account_id  = true
skip_s3_checksum            = true
use_path_style              = true
endpoints = { s3 = "${ep}" }
EOF
}

# Sets global TF_CMDS array and TF_PIPELINE_HAS_APPLY (0|1).
dockpipe_tf_parse_commands() {
  TF_CMDS=()
  TF_PIPELINE_HAS_APPLY=0
  local raw="${DOCKPIPE_TF_COMMANDS:-init,apply}"
  raw="${raw//[[:space:]]/}"
  [[ -z "$raw" ]] && dockpipe_tf_die "DOCKPIPE_TF_COMMANDS is empty (skip Terraform outside this helper)"

  local IFS=','
  local -a parts=()
  read -ra parts <<<"$raw"
  local p lower
  for p in "${parts[@]}"; do
    [[ -z "$p" ]] && continue
    lower=$(echo "$p" | tr '[:upper:]' '[:lower:]')
    case "$lower" in
    init) TF_CMDS+=("init") ;;
    plan) TF_CMDS+=("plan") ;;
    apply)
      TF_CMDS+=("apply")
      TF_PIPELINE_HAS_APPLY=1
      ;;
    validate) TF_CMDS+=("validate") ;;
    fmt) TF_CMDS+=("fmt") ;;
    import) TF_CMDS+=("import") ;;
    *) dockpipe_tf_die "DOCKPIPE_TF_COMMANDS: unknown token '$p' (use init, plan, apply, validate, fmt, import — comma-separated)" ;;
    esac
  done
  [[ ${#TF_CMDS[@]} -eq 0 ]] && dockpipe_tf_die "DOCKPIPE_TF_COMMANDS produced no steps"

  if [[ "${DOCKPIPE_TF_SKIP_INIT:-0}" != "1" ]]; then
    local has_init=false need_init=false
    for p in "${TF_CMDS[@]}"; do
      [[ "$p" == init ]] && has_init=true
      if [[ "$p" == plan || "$p" == apply || "$p" == validate || "$p" == fmt || "$p" == import ]]; then
        need_init=true
      fi
    done
    if $need_init && ! $has_init; then
      TF_CMDS=(init "${TF_CMDS[@]}")
    fi
  fi
}

# Run terraform workspace select/new when DOCKPIPE_TF_WORKSPACE is set (after init).
dockpipe_tf_maybe_workspace() {
  [[ -n "${DOCKPIPE_TF_WORKSPACE:-}" ]] || return 0
  terraform workspace select "${DOCKPIPE_TF_WORKSPACE}" 2>/dev/null || terraform workspace new "${DOCKPIPE_TF_WORKSPACE}"
}

# One or more imports: DOCKPIPE_TF_IMPORT_ARGS (single line) and/or DOCKPIPE_TF_IMPORT_FILE (one import per line).
# Line format: ADDRESS first token, ID is the remainder of the line (supports IDs with spaces).
dockpipe_tf_run_imports() {
  if [[ -n "${DOCKPIPE_TF_IMPORT_ARGS:-}" ]]; then
    local -a imp=()
    read -r -a imp <<<"${DOCKPIPE_TF_IMPORT_ARGS}"
    terraform import -input=false "${imp[@]}"
  fi
  if [[ -n "${DOCKPIPE_TF_IMPORT_FILE:-}" ]]; then
    local f="$DOCKPIPE_TF_IMPORT_FILE"
    [[ -f "$f" ]] || dockpipe_tf_die "DOCKPIPE_TF_IMPORT_FILE not found: $f"
    while IFS= read -r line || [[ -n "$line" ]]; do
      [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
      local addr id
      addr="${line%% *}"
      id="${line#* }"
      [[ -n "$addr" && -n "$id" ]] || dockpipe_tf_die "bad import line (want 'ADDRESS ID...'): $line"
      terraform import -input=false "$addr" "$id"
    done <"$f"
  fi
}

# Args: terraform_dir, path_for_remote_backend_hcl (written when backend=remote), optional skip_apply_hint (for dry-run messaging).
dockpipe_tf_run_pipeline() {
  local tf_dir="$1"
  local backend_cfg_path="${2:-}"
  local _hint="${3:-}"

  command -v terraform >/dev/null 2>&1 || dockpipe_tf_die "install Terraform (https://developer.hashicorp.com/terraform/downloads)"

  local tf_backend="${DOCKPIPE_TF_BACKEND:-local}"
  dockpipe_tf_parse_commands

  if [[ "${DOCKPIPE_TF_DRY_RUN:-0}" == "1" ]]; then
    dockpipe_tf_log "DRY RUN — Terraform in $tf_dir"
    dockpipe_tf_log "DRY RUN — DOCKPIPE_TF_COMMANDS → ${TF_CMDS[*]}"
    dockpipe_tf_log "DRY RUN — DOCKPIPE_TF_BACKEND=$tf_backend"
    if [[ "$tf_backend" == remote ]]; then
      dockpipe_tf_log "DRY RUN — state bucket: ${DOCKPIPE_TF_STATE_BUCKET:-dockpipe} key: ${DOCKPIPE_TF_STATE_KEY:-state/terraform.tfstate}"
    fi
    if [[ "$TF_PIPELINE_HAS_APPLY" != "1" && -n "$_hint" ]]; then
      dockpipe_tf_log "DRY RUN — $_hint"
    fi
    return 0
  fi

  if [[ "$tf_backend" == remote ]]; then
    if [[ -n "${DOCKPIPE_TF_STATE_ACCESS_KEY_ID:-}" && -n "${DOCKPIPE_TF_STATE_SECRET_ACCESS_KEY:-}" ]]; then
      :
    elif [[ -n "${R2_STATE_ACCESS_KEY_ID:-}" && -n "${R2_STATE_SECRET_ACCESS_KEY:-}" ]]; then
      :
    elif [[ -n "${AWS_ACCESS_KEY_ID:-}" && -n "${AWS_SECRET_ACCESS_KEY:-}" ]]; then
      :
    else
      dockpipe_tf_die "remote backend: set DOCKPIPE_TF_STATE_ACCESS_KEY_ID/SECRET or R2_STATE_* or AWS_* for state bucket; or DOCKPIPE_TF_BACKEND=local"
    fi
  fi

  local -a init_extra=() plan_extra=() apply_extra=() validate_extra=() fmt_extra=()
  [[ -n "${DOCKPIPE_TF_INIT_ARGS:-}" ]] && read -r -a init_extra <<<"${DOCKPIPE_TF_INIT_ARGS}"
  [[ -n "${DOCKPIPE_TF_PLAN_ARGS:-}" ]] && read -r -a plan_extra <<<"${DOCKPIPE_TF_PLAN_ARGS}"
  [[ -n "${DOCKPIPE_TF_APPLY_ARGS:-}" ]] && read -r -a apply_extra <<<"${DOCKPIPE_TF_APPLY_ARGS}"
  [[ -n "${DOCKPIPE_TF_VALIDATE_ARGS:-}" ]] && read -r -a validate_extra <<<"${DOCKPIPE_TF_VALIDATE_ARGS}"
  [[ -n "${DOCKPIPE_TF_FMT_ARGS:-}" ]] && read -r -a fmt_extra <<<"${DOCKPIPE_TF_FMT_ARGS}"

  (
    set -e
    export TF_IN_AUTOMATION=1
    cd "$tf_dir"

    export AWS_EC2_METADATA_DISABLED=true
    if [[ -n "${DOCKPIPE_TF_STATE_ACCESS_KEY_ID:-}" && -n "${DOCKPIPE_TF_STATE_SECRET_ACCESS_KEY:-}" ]]; then
      export AWS_ACCESS_KEY_ID="${DOCKPIPE_TF_STATE_ACCESS_KEY_ID}"
      export AWS_SECRET_ACCESS_KEY="${DOCKPIPE_TF_STATE_SECRET_ACCESS_KEY}"
    elif [[ -n "${R2_STATE_ACCESS_KEY_ID:-}" && -n "${R2_STATE_SECRET_ACCESS_KEY:-}" ]]; then
      export AWS_ACCESS_KEY_ID="${R2_STATE_ACCESS_KEY_ID}"
      export AWS_SECRET_ACCESS_KEY="${R2_STATE_SECRET_ACCESS_KEY}"
    fi

    local step did_init=false
    for step in "${TF_CMDS[@]}"; do
      case "$step" in
      init)
        if [[ "$tf_backend" == local ]]; then
          terraform init -input=false -backend=false "${init_extra[@]}"
        else
          [[ -n "$backend_cfg_path" ]] || dockpipe_tf_die "remote backend requires backend config path (arg 2 to dockpipe_tf_run_pipeline)"
          dockpipe_tf_write_r2_remote_backend_config "$backend_cfg_path"
          terraform init -input=false -backend-config="$backend_cfg_path" "${init_extra[@]}"
        fi
        did_init=true
        dockpipe_tf_maybe_workspace
        ;;
      plan)
        terraform plan -input=false "${plan_extra[@]}"
        ;;
      apply)
        if [[ "${DOCKPIPE_TF_APPLY_AUTO_APPROVE:-1}" == "1" ]]; then
          terraform apply -auto-approve -input=false "${apply_extra[@]}"
        else
          terraform apply -input=false "${apply_extra[@]}"
        fi
        ;;
      validate)
        terraform validate "${validate_extra[@]}"
        ;;
      fmt)
        terraform fmt "${fmt_extra[@]}"
        ;;
      import)
        if [[ -z "${DOCKPIPE_TF_IMPORT_ARGS:-}" && -z "${DOCKPIPE_TF_IMPORT_FILE:-}" ]]; then
          dockpipe_tf_die "import step requires DOCKPIPE_TF_IMPORT_ARGS and/or DOCKPIPE_TF_IMPORT_FILE"
        fi
        dockpipe_tf_run_imports
        ;;
      esac
    done
  ) || dockpipe_tf_die "terraform failed"
}
