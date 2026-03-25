#!/usr/bin/env bash
# Host: tar.gz a folder and upload to Cloudflare R2.
#
# Two credential modes:
#   1) S3-compatible API — AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY (R2 access keys).
#   2) Single Cloudflare API token — CLOUDFLARE_API_TOKEN + CLOUDFLARE_ACCOUNT_ID:
#      Terraform creates the bucket (optional; default for this mode), Wrangler uploads the object.
set -euo pipefail

ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
cd "$ROOT"

die() { echo "r2-publish: $*" >&2; exit 1; }

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
WORK="$(mktemp -d "${TMPDIR}/dockpipe-r2-publish.XXXXXX")"
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
  for d in "shipyard/workflows/r2-publish/terraform" "templates/r2-publish/terraform"; do
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

write_r2_tf_backend_config() {
  local out="$1"
  local bucket="${R2_TF_STATE_BUCKET:-dockpipe}"
  local key="${R2_TF_STATE_KEY:-state/r2-publish/terraform.tfstate}"
  local ep="https://${CLOUDFLARE_ACCOUNT_ID}.r2.cloudflarestorage.com"
  cat > "$out" <<EOF
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

run_terraform_apply() {
  local tf_dir="$1"
  command -v terraform >/dev/null 2>&1 || die "install Terraform (https://developer.hashicorp.com/terraform/downloads)"
  [[ -n "${CLOUDFLARE_API_TOKEN:-}" ]] || die "CLOUDFLARE_API_TOKEN required for Terraform"
  [[ -n "${CLOUDFLARE_ACCOUNT_ID:-}" ]] || die "CLOUDFLARE_ACCOUNT_ID required for Terraform"

  local tf_backend="${R2_TF_BACKEND:-remote}"
  local backend_cfg="${WORK}/r2-tf-backend.hcl"

  if [[ "${R2_PUBLISH_DRY_RUN:-0}" == "1" ]]; then
    echo "r2-publish: DRY RUN — would run terraform init && apply in $tf_dir"
    echo "r2-publish: DRY RUN — Terraform backend: ${tf_backend}"
    if [[ "$tf_backend" == remote ]]; then
      echo "r2-publish: DRY RUN — state bucket: ${R2_TF_STATE_BUCKET:-dockpipe} key: ${R2_TF_STATE_KEY:-state/r2-publish/terraform.tfstate}"
    fi
    return 0
  fi

  if [[ "$tf_backend" == remote ]]; then
    if [[ -n "${R2_STATE_ACCESS_KEY_ID:-}" && -n "${R2_STATE_SECRET_ACCESS_KEY:-}" ]]; then
      :
    elif [[ -n "${AWS_ACCESS_KEY_ID:-}" && -n "${AWS_SECRET_ACCESS_KEY:-}" ]]; then
      :
    else
      die "remote Terraform state on R2 requires R2_STATE_ACCESS_KEY_ID and R2_STATE_SECRET_ACCESS_KEY (bucket-scoped Object Read & Write for ${R2_TF_STATE_BUCKET:-dockpipe}), or reuse AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY for that bucket; or set R2_TF_BACKEND=local"
    fi
  fi

  (
    set -e
    export TF_IN_AUTOMATION=1
    export TF_VAR_cloudflare_api_token="${CLOUDFLARE_API_TOKEN}"
    export TF_VAR_account_id="${CLOUDFLARE_ACCOUNT_ID}"
    export TF_VAR_bucket_name="${R2_BUCKET}"
    if [[ -n "${R2_TF_LOCATION:-}" ]]; then
      export TF_VAR_location="${R2_TF_LOCATION}"
    else
      unset TF_VAR_location 2>/dev/null || true
    fi

    cd "$tf_dir"

    export AWS_EC2_METADATA_DISABLED=true
    if [[ -n "${R2_STATE_ACCESS_KEY_ID:-}" && -n "${R2_STATE_SECRET_ACCESS_KEY:-}" ]]; then
      export AWS_ACCESS_KEY_ID="${R2_STATE_ACCESS_KEY_ID}"
      export AWS_SECRET_ACCESS_KEY="${R2_STATE_SECRET_ACCESS_KEY}"
    fi

    if [[ "$tf_backend" == local ]]; then
      terraform init -input=false -backend=false
    else
      write_r2_tf_backend_config "$backend_cfg"
      terraform init -input=false -backend-config="$backend_cfg"
    fi

    terraform apply -auto-approve -input=false
  ) || die "terraform failed"
}

if should_run_terraform; then
  TF_DIR="$(find_terraform_dir)" || die "Terraform is required for this run but no module directory found. Copy shipyard/workflows/r2-publish/terraform into your project, set R2_TERRAFORM_DIR, or set R2_SKIP_TERRAFORM=1 if the bucket already exists."
  run_terraform_apply "$TF_DIR"
fi

ARCHIVE_PATH="${WORK}/${ARCHIVE_NAME}"
# Pack so extracting yields a single top-level directory named like the source folder.
tar -czf "$ARCHIVE_PATH" -C "$(dirname "$SRC_REL")" "$(basename "$SRC_REL")"

if [[ "${R2_PUBLISH_DRY_RUN:-0}" == "1" ]]; then
  echo "r2-publish: DRY RUN — would upload:"
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
  echo "r2-publish: uploaded s3://${BUCKET}/${KEY}"
  echo "r2-publish: endpoint ${ENDPOINT}"
else
  export CLOUDFLARE_API_TOKEN
  export CLOUDFLARE_ACCOUNT_ID
  run_wrangler_put "${BUCKET}/${KEY}" "$ARCHIVE_PATH"
  echo "r2-publish: uploaded r2://${BUCKET}/${KEY}"
fi
