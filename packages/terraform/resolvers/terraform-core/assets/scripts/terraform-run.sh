#!/usr/bin/env bash
# Provider-agnostic Terraform host: terraform-pipeline.sh + DOCKPIPE_TF_* only.
# Shipped with packages/terraform/resolvers/terraform-core (dockpipe.terraform.core), not src/core.
# Cloudflare R2 / generated R2 state: use packages/cloud/storage/.../terraform-cloudflare-r2-run.sh (dockpipe.cloudflare.r2infra).
set -euo pipefail

eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"
dockpipe_sdk init-script

if [[ "${DOCKPIPE_TF_OPTIONAL_WHEN_UNSET:-0}" == "1" ]]; then
  case "${DOCKPIPE_TF_COMMANDS:-}" in
    *[![:space:]]*) ;;
    *)
      echo "${WF_NS}: Terraform skipped (DOCKPIPE_TF_OPTIONAL_WHEN_UNSET=1 and DOCKPIPE_TF_COMMANDS unset or empty)" >&2
      exit 0
      ;;
  esac
fi

[[ -n "${DOCKPIPE_TF_MODULE_DIR:-}" ]] || dockpipe_sdk die "set DOCKPIPE_TF_MODULE_DIR to your Terraform root (repo-relative or absolute)"

tf_dir="${DOCKPIPE_TF_MODULE_DIR}"
[[ "$tf_dir" != /* ]] && tf_dir="$ROOT/$tf_dir"
[[ -d "$tf_dir" ]] || dockpipe_sdk die "not a directory: $tf_dir"

dockpipe_sdk source terraform-pipeline || dockpipe_sdk die "could not source terraform pipeline via dockpipe sdk"
export DOCKPIPE_TF_LOG_PREFIX="${DOCKPIPE_TF_LOG_PREFIX:-${WF_NS}}"
dockpipe_tf_map_generic_env

tf_backend="${DOCKPIPE_TF_BACKEND:-local}"
backend_arg=""
if [[ "$tf_backend" == remote ]]; then
  [[ -n "${DOCKPIPE_TF_REMOTE_BACKEND_FILE:-}" ]] || dockpipe_sdk die "DOCKPIPE_TF_BACKEND=remote requires DOCKPIPE_TF_REMOTE_BACKEND_FILE (path to an existing backend HCL). Generated Cloudflare R2 backends are only in dockpipe.cloudflare.r2infra / terraform-cloudflare-r2-run.sh"
  [[ -f "${DOCKPIPE_TF_REMOTE_BACKEND_FILE}" ]] || dockpipe_sdk die "DOCKPIPE_TF_REMOTE_BACKEND_FILE not found: ${DOCKPIPE_TF_REMOTE_BACKEND_FILE}"
  backend_arg="unused"
fi

dockpipe_tf_run_pipeline "$tf_dir" "$backend_arg" "${DOCKPIPE_TF_PIPELINE_HINT:-}"
