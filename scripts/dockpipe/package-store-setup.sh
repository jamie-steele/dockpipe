#!/usr/bin/env bash
# Used by workflows/package-store-infra: compile local package store, pack release/artifacts, print manifest preview.
set -euo pipefail

ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
cd "$ROOT"
ROOT="$(pwd -P)"

BIN="${DOCKPIPE_BIN:-dockpipe}"
if [[ -x "${ROOT}/src/bin/dockpipe" ]]; then
  BIN="${ROOT}/src/bin/dockpipe"
fi

OUT="${PACKAGE_STORE_OUT:-${ROOT}/release/artifacts}"
# r2-publish expects R2_PUBLISH_SOURCE relative to workdir (e.g. release/artifacts).
if [[ "$OUT" == "$ROOT"/* ]]; then
  REL_OUT="${OUT#"$ROOT"/}"
else
  REL_OUT="$OUT"
fi

# Publish + Terraform hints (workflow vars → env). Canonical state vars: DOCKPIPE_TF_* (terraform-pipeline.sh).
# Legacy R2_TF_* still accepted by r2publish’s mapper when DOCKPIPE_TF_* is unset.
R2_PREFIX="${R2_PREFIX:-packages/}"
[[ -n "$R2_PREFIX" && "${R2_PREFIX: -1}" != / ]] && R2_PREFIX="${R2_PREFIX}/"
DOCKPIPE_TF_BACKEND="${DOCKPIPE_TF_BACKEND:-${R2_TF_BACKEND:-remote}}"
DOCKPIPE_TF_STATE_BUCKET="${DOCKPIPE_TF_STATE_BUCKET:-${R2_TF_STATE_BUCKET:-dockpipe}}"
DOCKPIPE_TF_STATE_KEY="${DOCKPIPE_TF_STATE_KEY:-${R2_TF_STATE_KEY:-state/terraform.tfstate}}"
R2_SKIP_TERRAFORM="${R2_SKIP_TERRAFORM:-0}"

echo "[package-store-infra] repo root: ${ROOT}"
echo "[package-store-infra] CLI: ${BIN}"
echo "[package-store-infra] planned object key prefix (R2_PREFIX): ${R2_PREFIX}"
echo "[package-store-infra] Terraform state (DOCKPIPE_TF_*): backend=${DOCKPIPE_TF_BACKEND} bucket=${DOCKPIPE_TF_STATE_BUCKET} key=${DOCKPIPE_TF_STATE_KEY}"
echo "[package-store-infra] R2_SKIP_TERRAFORM=${R2_SKIP_TERRAFORM} (r2publish: 1 skips Terraform if bucket already exists; import if TF tracks resources)"
echo "[package-store-infra] (1/2) dockpipe build — core, resolver packages, workflow packages → .dockpipe/internal/packages/"
"$BIN" build

echo "[package-store-infra] (2/2) dockpipe package build store — tarballs + packages-store-manifest.json → ${OUT}"
"$BIN" package build store --workdir "$ROOT" --out "$OUT"

echo ""
echo "[package-store-infra] --- ${OUT} (package manager / HTTPS origin layout) ---"
ls -la "$OUT" 2>/dev/null || {
  echo "[package-store-infra] warning: output dir missing" >&2
  exit 1
}
n_tar=$(find "$OUT" -maxdepth 1 -name '*.tar.gz' -type f 2>/dev/null | wc -l)
echo "[package-store-infra] gzip tarballs in this directory: ${n_tar}"

if [[ -f "${OUT}/packages-store-manifest.json" ]]; then
  echo ""
  echo "[package-store-infra] packages-store-manifest.json (first 50 lines):"
  head -n 50 "${OUT}/packages-store-manifest.json"
  echo ""
fi

echo "[package-store-infra] Next: publish ${OUT} (same vars as above; credentials from vault / env):"
echo "  R2_PUBLISH_SOURCE=\"${REL_OUT}\" R2_PREFIX=\"${R2_PREFIX}\" \\"
echo "  DOCKPIPE_TF_BACKEND=\"${DOCKPIPE_TF_BACKEND}\" DOCKPIPE_TF_STATE_BUCKET=\"${DOCKPIPE_TF_STATE_BUCKET}\" \\"
echo "  DOCKPIPE_TF_STATE_KEY=\"${DOCKPIPE_TF_STATE_KEY}\" R2_SKIP_TERRAFORM=\"${R2_SKIP_TERRAFORM}\" \\"
echo "  ${BIN} --workflow dockpipe.cloudflare.r2publish --workdir \"${ROOT}\" --"
echo "[package-store-infra] State credentials: DOCKPIPE_TF_STATE_ACCESS_KEY_ID/SECRET, R2_STATE_*, or AWS_* (see terraform-pipeline.sh). Dry-run: R2_PUBLISH_DRY_RUN=1. Skip vault: DOCKPIPE_OP_INJECT=0 or --no-op-inject."
