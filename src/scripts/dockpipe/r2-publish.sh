#!/usr/bin/env bash
# Host: tar.gz a folder and upload to Cloudflare R2.
#
# Two credential modes:
#   1) S3-compatible API — AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY (R2 access keys).
#   2) Single Cloudflare API token — CLOUDFLARE_API_TOKEN + CLOUDFLARE_ACCOUNT_ID:
#      Terraform creates the bucket (optional; default for this mode), Wrangler uploads the object.
#
# Terraform (when enabled): uses templates/core/assets/scripts/terraform-pipeline.sh (DOCKPIPE_TF_*).
# Legacy R2_TERRAFORM_* env vars are mapped automatically. See docs/terraform-pipeline.md.
set -euo pipefail

# Workflow id (matches workflows/*/config.yml name: dockpipe.cloudflare.r2publish)
WF_NS="${DOCKPIPE_WORKFLOW_NAME:-dockpipe.cloudflare.r2publish}"

ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
cd "$ROOT"

die() { echo "${WF_NS}: $*" >&2; exit 1; }

SRC_REL="${R2_PUBLISH_SOURCE:-release/artifacts}"
[[ -d "$SRC_REL" ]] || die "source directory missing: $SRC_REL (set R2_PUBLISH_SOURCE or mkdir -p release/artifacts)"

BUCKET="${R2_BUCKET:-}"
[[ -n "$BUCKET" ]] || die "set R2_BUCKET (R2 bucket name)"

# --- upload mode -----------------------------------------------------------
UPLOAD_MODE=""
if [[ -n "${AWS_ACCESS_KEY_ID:-}" && -n "${AWS_SECRET_ACCESS_KEY:-}" ]]; then
  UPLOAD_MODE="s3"
elif [[ -n "${CLOUDFLARE_API_TOKEN:-}" ]]; then
  UPLOAD_MODE="wrangler"
else
  die "set R2 bucket credentials: either AWS_ACCESS_KEY_ID+AWS_SECRET_ACCESS_KEY (R2 S3 keys) or CLOUDFLARE_API_TOKEN (API token + Wrangler/Terraform)"
fi

if [[ "$UPLOAD_MODE" == wrangler ]]; then
  [[ -n "${CLOUDFLARE_ACCOUNT_ID:-}" ]] || die "set CLOUDFLARE_ACCOUNT_ID (required for CLOUDFLARE_API_TOKEN mode)"
fi

# Endpoint: full S3 API URL for your account (S3 mode only).
ENDPOINT="${R2_ENDPOINT_URL:-${AWS_ENDPOINT_URL_S3:-}}"
if [[ "$UPLOAD_MODE" == s3 ]]; then
  if [[ -z "$ENDPOINT" ]]; then
    ACCT="${CLOUDFLARE_ACCOUNT_ID:-${R2_ACCOUNT_ID:-}}"
    [[ -n "$ACCT" ]] || die "set R2_ENDPOINT_URL (https://<account_id>.r2.cloudflarestorage.com) or CLOUDFLARE_ACCOUNT_ID"
    ENDPOINT="https://${ACCT}.r2.cloudflarestorage.com"
  fi
fi

PREFIX="${R2_PREFIX:-}"
[[ -n "$PREFIX" && "${PREFIX: -1}" != / ]] && PREFIX="${PREFIX}/"

STAMP="$(date +%Y%m%d-%H%M%S)"
ARCHIVE_NAME="${R2_ARCHIVE_NAME:-dockpipe-publish-${STAMP}.tar.gz}"
TMPDIR="${TMPDIR:-/tmp}"
WORK="$(mktemp -d "${TMPDIR}/dockpipe-cf-r2publish.XXXXXX")"
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

KEY="${PREFIX}${ARCHIVE_NAME}"

find_terraform_dir() {
  if [[ -n "${R2_TERRAFORM_DIR:-}" ]]; then
    local d="$R2_TERRAFORM_DIR"
    if [[ "$d" != /* ]]; then
      d="$ROOT/$d"
    fi
    if [[ -d "$d" ]]; then
      echo "$d"
      return 0
    fi
    return 1
  fi
  for d in \
    ".staging/packages/dockpipe/cloud/storage/resolvers/r2/dockpipe.cloudflare.r2publish/terraform" \
    "templates/dockpipe.cloudflare.r2publish/terraform" \
    "templates/r2-publish/terraform"; do
    if [[ -d "$ROOT/$d" ]]; then
      echo "$ROOT/$d"
      return 0
    fi
  done
  return 1
}

should_run_terraform() {
  if [[ "${R2_SKIP_TERRAFORM:-0}" == "1" ]]; then
    return 1
  fi
  if [[ "${R2_USE_TERRAFORM:-}" == "0" ]]; then
    return 1
  fi
  if [[ "${R2_USE_TERRAFORM:-}" == "1" ]]; then
    return 0
  fi
  if [[ "$UPLOAD_MODE" == wrangler ]]; then
    return 0
  fi
  return 1
}

source_terraform_pipeline_lib() {
  local candidate
  for candidate in "$ROOT/templates/core/assets/scripts/terraform-pipeline.sh" "$ROOT/src/core/assets/scripts/terraform-pipeline.sh"; do
    if [[ -f "$candidate" ]]; then
      # shellcheck source=/dev/null
      source "$candidate"
      return 0
    fi
  done
  die "terraform-pipeline.sh not found — expected templates/core/assets/scripts/terraform-pipeline.sh (dockpipe init) or src/core/assets/scripts/ in this repo"
}

run_terraform_pipeline() {
  local tf_dir="$1"
  command -v terraform >/dev/null 2>&1 || die "install Terraform (https://developer.hashicorp.com/terraform/downloads)"
  [[ -n "${CLOUDFLARE_API_TOKEN:-}" ]] || die "CLOUDFLARE_API_TOKEN required for Terraform"
  [[ -n "${CLOUDFLARE_ACCOUNT_ID:-}" ]] || die "CLOUDFLARE_ACCOUNT_ID required for Terraform"

  source_terraform_pipeline_lib
  export DOCKPIPE_TF_LOG_PREFIX="${DOCKPIPE_TF_LOG_PREFIX:-${WF_NS}}"
  dockpipe_tf_map_r2_publish_env

  export TF_IN_AUTOMATION=1
  export TF_VAR_cloudflare_api_token="${CLOUDFLARE_API_TOKEN}"
  export TF_VAR_account_id="${CLOUDFLARE_ACCOUNT_ID}"
  export TF_VAR_bucket_name="${R2_BUCKET}"
  if [[ -n "${R2_TF_LOCATION:-}" ]]; then
    export TF_VAR_location="${R2_TF_LOCATION}"
  else
    unset TF_VAR_location 2>/dev/null || true
  fi

  dockpipe_tf_run_pipeline "$tf_dir" "$WORK/r2-tf-backend.hcl" "would skip tarball upload after Terraform (no apply step; R2_PUBLISH_ALWAYS_UPLOAD=1 to force upload)"
}

TERRAFORM_RAN=false
TF_PIPELINE_HAS_APPLY=0

if should_run_terraform; then
  TF_DIR="$(find_terraform_dir)" || die "Terraform is required for this run but no module directory found. Copy .staging/packages/dockpipe/cloud/storage/resolvers/r2/dockpipe.cloudflare.r2publish/terraform into your project, set R2_TERRAFORM_DIR, or set R2_SKIP_TERRAFORM=1 if the bucket already exists."
  run_terraform_pipeline "$TF_DIR"
  TERRAFORM_RAN=true
fi

# No tarball upload unless Terraform ran apply (or user forces upload). Default R2_TERRAFORM_COMMANDS is init,apply.
if [[ "$TERRAFORM_RAN" == true ]] && [[ "$TF_PIPELINE_HAS_APPLY" != "1" ]] && [[ "${R2_PUBLISH_ALWAYS_UPLOAD:-0}" != "1" ]]; then
  echo "${WF_NS}: skipping tarball upload (R2_TERRAFORM_COMMANDS has no apply step; set R2_PUBLISH_ALWAYS_UPLOAD=1 to upload anyway)"
  exit 0
fi

ARCHIVE_PATH="${WORK}/${ARCHIVE_NAME}"
# Pack so extracting yields a single top-level directory named like the source folder.
tar -czf "$ARCHIVE_PATH" -C "$(dirname "$SRC_REL")" "$(basename "$SRC_REL")"

if [[ "${R2_PUBLISH_DRY_RUN:-0}" == "1" ]]; then
  echo "${WF_NS}: DRY RUN — would upload:"
  echo "  file:    $ARCHIVE_PATH ($(wc -c <"$ARCHIVE_PATH") bytes)"
  if [[ "$UPLOAD_MODE" == s3 ]]; then
    echo "  s3://$BUCKET/$KEY"
    echo "  endpoint: $ENDPOINT"
  else
    echo "  r2 object: ${BUCKET}/${KEY} (wrangler)"
    echo "  account: ${CLOUDFLARE_ACCOUNT_ID}"
  fi
  exit 0
fi

run_wrangler_put() {
  local object_path="$1"
  local file_path="$2"
  local ct="${R2_CONTENT_TYPE:-application/gzip}"
  if command -v wrangler >/dev/null 2>&1; then
    wrangler r2 object put "$object_path" --file "$file_path" --content-type "$ct"
  else
    command -v npx >/dev/null 2>&1 || die "install wrangler or Node.js (npx) for CLOUDFLARE_API_TOKEN uploads"
    npx --yes wrangler@3 r2 object put "$object_path" --file "$file_path" --content-type "$ct"
  fi
}

if [[ "$UPLOAD_MODE" == s3 ]]; then
  export AWS_EC2_METADATA_DISABLED=true
  aws s3 cp "$ARCHIVE_PATH" "s3://${BUCKET}/${KEY}" \
    --endpoint-url "$ENDPOINT" \
    --region auto
  echo "${WF_NS}: uploaded s3://${BUCKET}/${KEY}"
  echo "${WF_NS}: endpoint ${ENDPOINT}"
else
  export CLOUDFLARE_API_TOKEN
  export CLOUDFLARE_ACCOUNT_ID
  run_wrangler_put "${BUCKET}/${KEY}" "$ARCHIVE_PATH"
  echo "${WF_NS}: uploaded r2://${BUCKET}/${KEY}"
fi
