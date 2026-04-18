#!/usr/bin/env bash
# Host: tar.gz a folder and upload to Cloudflare R2; optionally Terraform first.
#
# Modes:
#   R2_INFRA_ONLY=1     — Terraform only, then exit (no package tarball upload). Remote state still uses
#                         the S3 backend (R2); state is written on plan/apply — not on init alone.
#   DOCKPIPE_TF_OPTIONAL_WHEN_UNSET=1 — (R2_INFRA_ONLY only) if DOCKPIPE_TF_COMMANDS is unset or whitespace-only,
#                         exit 0 before Terraform (compose parent workflows without --tf; use --tf plan to run).
#   R2_SKIP_TERRAFORM=1 — Skip Terraform; tar + upload only (bucket must already exist).
#   (default)           — Terraform when enabled, then tar + upload (wrangler mode runs TF by default).
#
# Two credential modes for upload:
#   1) S3-compatible API — AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY (R2 access keys).
#   2) Single Cloudflare API token — CLOUDFLARE_API_TOKEN + CLOUDFLARE_ACCOUNT_ID:
#      Terraform creates the bucket (optional; default for this mode), Wrangler uploads the object.
#
# Terraform runs through the dockpipe.cloudflare.r2infra workflow when enabled.
set -euo pipefail

# Workflow id (matches workflows/*/config.yml name)
WF_NS="${DOCKPIPE_WORKFLOW_NAME:-dockpipe.cloudflare.r2publish}"

ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
cd "$ROOT"

die() { echo "${WF_NS}: $*" >&2; exit 1; }

# Same as terraform-pipeline.sh: accept 32-char id or full https://<id>.r2.cloudflarestorage.com (dashboard paste).
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

resolve_dockpipe_bin() {
  if [[ -n "${DOCKPIPE_BIN:-}" ]]; then
    printf '%s\n' "$DOCKPIPE_BIN"
    return 0
  fi
  if [[ -x "$ROOT/src/bin/dockpipe" ]]; then
    printf '%s\n' "$ROOT/src/bin/dockpipe"
    return 0
  fi
  if command -v dockpipe >/dev/null 2>&1; then
    command -v dockpipe
    return 0
  fi
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
  if [[ "${UPLOAD_MODE:-}" == wrangler ]]; then
    return 0
  fi
  return 1
}

run_terraform_pipeline() {
  local backend_hcl="$1"
  local dockpipe_bin
  dockpipe_bin="$(resolve_dockpipe_bin)" || die "dockpipe not found; set DOCKPIPE_BIN or add dockpipe to PATH"
  if [[ -n "${CLOUDFLARE_API_TOKEN:-}" ]]; then
    :
  elif [[ -n "${CLOUDFLARE_EMAIL:-}" && -n "${CLOUDFLARE_GLOBAL_API_KEY:-}" ]]; then
    :
  else
    die "Terraform auth: set CLOUDFLARE_API_TOKEN or (CLOUDFLARE_EMAIL + CLOUDFLARE_GLOBAL_API_KEY)"
  fi
  [[ -n "${CLOUDFLARE_ACCOUNT_ID:-}" ]] || die "CLOUDFLARE_ACCOUNT_ID required for Terraform"
  export DOCKPIPE_TF_BACKEND_HCL_PATH="$backend_hcl"
  export DOCKPIPE_TF_ATTACH_CLOUDFLARE_PROVIDER=1
  export DOCKPIPE_TF_LOG_PREFIX="${DOCKPIPE_TF_LOG_PREFIX:-${WF_NS}}"
  export DOCKPIPE_TF_PIPELINE_HINT="would skip tarball upload after Terraform (no apply step; R2_PUBLISH_ALWAYS_UPLOAD=1 to force upload)"
  "$dockpipe_bin" --workflow dockpipe.cloudflare.r2infra --workdir "$ROOT" --
}

# --- Infra only: Terraform then exit (no source dir, no tarball, no upload) -----------------
if [[ "${R2_INFRA_ONLY:-0}" == "1" ]]; then
  if [[ "${DOCKPIPE_TF_OPTIONAL_WHEN_UNSET:-0}" == "1" ]]; then
    case "${DOCKPIPE_TF_COMMANDS:-}" in
      *[![:space:]]*) ;;
      *)
        echo "${WF_NS}: infra-only Terraform skipped (DOCKPIPE_TF_OPTIONAL_WHEN_UNSET=1 and DOCKPIPE_TF_COMMANDS unset or empty; pass --tf plan or set DOCKPIPE_TF_COMMANDS)" >&2
        exit 0
        ;;
    esac
  fi
  if [[ "${R2_SKIP_TERRAFORM:-0}" == "1" ]]; then
    die "R2_INFRA_ONLY=1 conflicts with R2_SKIP_TERRAFORM=1"
  fi
  BUCKET="${R2_BUCKET:-}"
  [[ -n "$BUCKET" ]] || die "set R2_BUCKET"
  if [[ -n "${CLOUDFLARE_API_TOKEN:-}" ]]; then
    :
  elif [[ -n "${CLOUDFLARE_EMAIL:-}" && -n "${CLOUDFLARE_GLOBAL_API_KEY:-}" ]]; then
    :
  else
    die "Terraform auth: set CLOUDFLARE_API_TOKEN or (CLOUDFLARE_EMAIL + CLOUDFLARE_GLOBAL_API_KEY)"
  fi
  [[ -n "${CLOUDFLARE_ACCOUNT_ID:-}" ]] || die "CLOUDFLARE_ACCOUNT_ID required for Terraform"
  TMPDIR="${TMPDIR:-/tmp}"
  WORK="$(mktemp -d "${TMPDIR}/dockpipe-cf-r2infra.XXXXXX")"
  cleanup() { rm -rf "$WORK"; }
  trap cleanup EXIT
  run_terraform_pipeline "$WORK/r2-tf-backend.hcl"
  echo "${WF_NS}: infra-only done — no package tarball upload (use dockpipe.cloudflare.r2upload for that)."
  echo "${WF_NS}: Terraform remote state → s3 backend bucket=${DOCKPIPE_TF_STATE_BUCKET:-dockpipe} key=${DOCKPIPE_TF_STATE_KEY:-state/terraform.tfstate}; object appears in R2 after terraform apply (plan/init do not write the state blob)."
  exit 0
fi

# --- Publish path: needs source tree to tar ---------------------------------------------------
SRC_REL="${R2_PUBLISH_SOURCE:-release/artifacts}"
[[ -d "$SRC_REL" ]] || die "source directory missing: $SRC_REL (set R2_PUBLISH_SOURCE or mkdir -p release/artifacts)"

BUCKET="${R2_BUCKET:-}"
[[ -n "$BUCKET" ]] || die "set R2_BUCKET (R2 bucket name)"

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

ENDPOINT="${R2_ENDPOINT_URL:-${AWS_ENDPOINT_URL_S3:-}}"
if [[ "$UPLOAD_MODE" == s3 ]]; then
  if [[ -z "$ENDPOINT" ]]; then
    ACCT="${CLOUDFLARE_ACCOUNT_ID:-${R2_ACCOUNT_ID:-}}"
    [[ -n "$ACCT" ]] || die "set R2_ENDPOINT_URL (https://<account_id>.r2.cloudflarestorage.com) or CLOUDFLARE_ACCOUNT_ID"
    ACCT="$(dockpipe_r2_normalize_account_id "$ACCT")"
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

TERRAFORM_RAN=false
TF_PIPELINE_HAS_APPLY=0

if should_run_terraform; then
  case ",${DOCKPIPE_TF_COMMANDS:-${R2_TERRAFORM_COMMANDS:-init,apply}}," in
    *,apply,*) TF_PIPELINE_HAS_APPLY=1 ;;
  esac
  run_terraform_pipeline "$WORK/r2-tf-backend.hcl"
  TERRAFORM_RAN=true
fi

if [[ "$TERRAFORM_RAN" == true ]] && [[ "$TF_PIPELINE_HAS_APPLY" != "1" ]] && [[ "${R2_PUBLISH_ALWAYS_UPLOAD:-0}" != "1" ]]; then
  echo "${WF_NS}: skipping tarball upload (R2_TERRAFORM_COMMANDS has no apply step; set R2_PUBLISH_ALWAYS_UPLOAD=1 to upload anyway)"
  exit 0
fi

ARCHIVE_PATH="${WORK}/${ARCHIVE_NAME}"
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
