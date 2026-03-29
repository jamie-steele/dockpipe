#!/usr/bin/env bash
# Used by workflows/package-store-infra: compile local package store, pack release/artifacts, print manifest preview.
set -euo pipefail

ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
cd "$ROOT"

BIN="${DOCKPIPE_BIN:-dockpipe}"
if [[ -x "${ROOT}/src/bin/dockpipe" ]]; then
  BIN="${ROOT}/src/bin/dockpipe"
fi

OUT="${PACKAGE_STORE_OUT:-${ROOT}/release/artifacts}"

echo "[package-store-infra] repo root: ${ROOT}"
echo "[package-store-infra] CLI: ${BIN}"
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

echo "[package-store-infra] Next: publish to R2 (or any static host) so consumers can use install base URL:"
echo "  ${BIN} --workflow dockpipe.cloudflare.r2publish --workdir \"${ROOT}\" --"
echo "[package-store-infra] Dry-run only: R2_PUBLISH_DRY_RUN=1 ...  Skip vault: --no-op-inject if needed."
