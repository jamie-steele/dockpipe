#!/usr/bin/env bash
# dockpipe.cloudflare.r2infra — thin host: Cloudflare/R2 env + bundled Terraform module path, then
# terraform-core's terraform-pipeline.sh (DOCKPIPE_TF_* / R2_* mapping). Not provider-agnostic terraform-run.sh.
# Terraform module (.tf) lives with this infra workflow resolver.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/repo-tools.sh
source "$SCRIPT_DIR/lib/repo-tools.sh"
WF_NS="${DOCKPIPE_WORKFLOW_NAME:-dockpipe.cloudflare.r2infra}"
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
cd "$ROOT"

die() { echo "${WF_NS}: $*" >&2; exit 1; }

set_if_unset_from_pipelang() {
  local target="${1:-}"
  local source_name="${2:-}"
  [[ -n "$target" && -n "$source_name" ]] || return 0
  local source_val="${!source_name:-}"
  local target_val="${!target:-}"
  if [[ -z "$target_val" && -n "$source_val" ]]; then
    export "$target=$source_val"
  fi
}

source_pipelang_defaults_if_present() {
  local p=""
  local candidate
  for candidate in \
    "$SCRIPT_DIR/../../models/.pipelang/IR2InfraConfig.R2InfraConfig.bindings.env" \
    "$SCRIPT_DIR/../../models/.pipelang/R2InfraConfig.R2InfraConfig.bindings.env" \
    "$SCRIPT_DIR/../../models/.pipelang/r2-infra-config.R2InfraConfig.bindings.env"
  do
    if [[ -f "$candidate" ]]; then
      p="$candidate"
      break
    fi
  done
  [[ -n "$p" ]] || return 0
  # shellcheck source=/dev/null
  source "$p"
  local key target
  while IFS= read -r key; do
    [[ "$key" == PIPELANG_* ]] || continue
    target="${key#PIPELANG_}"
    set_if_unset_from_pipelang "$target" "$key"
  done < <(compgen -A variable PIPELANG_)
}

dockpipe_r2_normalize_account_id() {
  local raw="$1"
  while [[ "$raw" == https://* ]] || [[ "$raw" == http://* ]]; do
    raw="${raw#https://}"
    raw="${raw#http://}"
  done
  raw="${raw%%/*}"
  raw="${raw%%\?*}"
  if [[ "$raw" =~ ^([a-fA-F0-9]{32})\.r2\.cloudflarestorage\.com$ ]]; then
    echo "${BASH_REMATCH[1]}"
    return 0
  fi
  if [[ "$raw" =~ ^[a-fA-F0-9]{32}$ ]]; then
    echo "$raw"
    return 0
  fi
  echo "$raw"
}

source_terraform_pipeline_lib() {
  local dockpipe_bin="${DOCKPIPE_BIN:-}"
  if [[ -z "$dockpipe_bin" ]]; then
    dockpipe_bin="$(cloud_r2infra_resolve_dockpipe_bin "$ROOT")"
  fi
  [[ -n "$dockpipe_bin" ]] || die "dockpipe not found; set DOCKPIPE_BIN or add dockpipe to PATH"
  local pipeline_sh
  pipeline_sh="$("$dockpipe_bin" terraform pipeline-path 2>/dev/null)" || die "could not resolve terraform pipeline path via dockpipe terraform pipeline-path"
  [[ -f "$pipeline_sh" ]] || die "terraform pipeline script not found at ${pipeline_sh:-<empty>}"
  # shellcheck source=/dev/null
  source "$pipeline_sh"
}

find_terraform_dir() {
  if [[ -n "${DOCKPIPE_TF_MODULE_DIR:-}" ]]; then
    local d="${DOCKPIPE_TF_MODULE_DIR}"
    [[ "$d" != /* ]] && d="$ROOT/$d"
    if [[ -d "$d" ]]; then
      echo "$d"
      return 0
    fi
    return 1
  fi
  if [[ -n "${R2_TERRAFORM_DIR:-}" ]]; then
    local d2="${R2_TERRAFORM_DIR}"
    [[ "$d2" != /* ]] && d2="$ROOT/$d2"
    if [[ -d "$d2" ]]; then
      echo "$d2"
      return 0
    fi
    return 1
  fi
  local bundled="${SCRIPT_DIR}/../../terraform"
  if [[ -d "$bundled" ]]; then
    echo "$bundled"
    return 0
  fi
  return 1
}

source_pipelang_defaults_if_present

if [[ "${DOCKPIPE_TF_OPTIONAL_WHEN_UNSET:-0}" == "1" ]]; then
  case "${DOCKPIPE_TF_COMMANDS:-}" in
    *[![:space:]]*) ;;
    *)
      echo "${WF_NS}: Terraform skipped (DOCKPIPE_TF_OPTIONAL_WHEN_UNSET=1 and DOCKPIPE_TF_COMMANDS unset or empty)" >&2
      exit 0
      ;;
  esac
fi

attach_cloudflare_provider() {
  if [[ "${DOCKPIPE_TF_ATTACH_CLOUDFLARE_PROVIDER:-0}" != "1" ]]; then
    return 0
  fi
  if [[ -n "${CLOUDFLARE_API_TOKEN:-}" ]]; then
    :
  elif [[ -n "${CLOUDFLARE_EMAIL:-}" && -n "${CLOUDFLARE_GLOBAL_API_KEY:-}" ]]; then
    :
  else
    die "Terraform auth: set CLOUDFLARE_API_TOKEN or (CLOUDFLARE_EMAIL + CLOUDFLARE_GLOBAL_API_KEY)"
  fi
  [[ -n "${CLOUDFLARE_ACCOUNT_ID:-}" ]] || die "CLOUDFLARE_ACCOUNT_ID required for Terraform"
  [[ -n "${R2_BUCKET:-}" ]] || die "set R2_BUCKET (TF_VAR_bucket_name for Cloudflare R2 module)"

  local acct_id
  acct_id="$(dockpipe_r2_normalize_account_id "${CLOUDFLARE_ACCOUNT_ID}")"
  [[ "$acct_id" =~ ^[a-fA-F0-9]{32}$ ]] || die "CLOUDFLARE_ACCOUNT_ID must be the 32-char account id (not https://…r2.cloudflarestorage.com). Update .env or vault template."
  export CLOUDFLARE_ACCOUNT_ID="${acct_id}"

  export TF_IN_AUTOMATION=1
  unset TF_VAR_cloudflare_api_token TF_VAR_cloudflare_email TF_VAR_cloudflare_api_key 2>/dev/null || true
  if [[ -n "${CLOUDFLARE_API_TOKEN:-}" ]]; then
    export TF_VAR_cloudflare_api_token="${CLOUDFLARE_API_TOKEN}"
  else
    export TF_VAR_cloudflare_email="${CLOUDFLARE_EMAIL}"
    export TF_VAR_cloudflare_api_key="${CLOUDFLARE_GLOBAL_API_KEY}"
  fi
  export TF_VAR_account_id="${acct_id}"
  export TF_VAR_bucket_name="${R2_BUCKET}"
  if [[ -n "${R2_TF_LOCATION:-}" ]]; then
    export TF_VAR_location="${R2_TF_LOCATION}"
  else
    unset TF_VAR_location 2>/dev/null || true
  fi

  if [[ "${DOCKPIPE_TF_IMPORT_R2_BUCKET:-0}" == "1" ]] || [[ "${DOCKPIPE_TF_IMPORT_R2_BUCKET:-}" == "true" ]]; then
    if [[ -z "${DOCKPIPE_TF_IMPORT_ARGS:-}" ]]; then
      local jur="${DOCKPIPE_TF_R2_IMPORT_JURISDICTION:-default}"
      export DOCKPIPE_TF_IMPORT_ARGS="cloudflare_r2_bucket.publish ${acct_id}/${R2_BUCKET}/${jur}"
      echo "${WF_NS}: DOCKPIPE_TF_IMPORT_R2_BUCKET → import id ${acct_id}/${R2_BUCKET}/${jur}" >&2
    fi
  fi
}

attach_cloudflare_provider

TF_DIR="$(find_terraform_dir)" || die "Terraform module not found — set DOCKPIPE_TF_MODULE_DIR or R2_TERRAFORM_DIR, or keep the bundled module in this package"

source_terraform_pipeline_lib
export DOCKPIPE_TF_LOG_PREFIX="${DOCKPIPE_TF_LOG_PREFIX:-${WF_NS}}"

dockpipe_tf_map_r2_publish_env

tf_backend="${DOCKPIPE_TF_BACKEND:-local}"
backend_arg="${DOCKPIPE_TF_BACKEND_HCL_PATH:-}"
if [[ "$tf_backend" == remote ]]; then
  if [[ -n "${DOCKPIPE_TF_REMOTE_BACKEND_FILE:-}" ]]; then
    backend_arg="unused"
  elif [[ -z "$backend_arg" ]]; then
    backend_arg="$(mktemp "${TMPDIR:-/tmp}/dockpipe-tf-backend.XXXXXX")"
    cleanup_backend() { rm -f "$backend_arg"; }
    trap cleanup_backend EXIT
  fi
fi

dockpipe_tf_run_pipeline "$TF_DIR" "$backend_arg" "${DOCKPIPE_TF_PIPELINE_HINT:-}"
