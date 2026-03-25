#!/usr/bin/env bash
# Build release/artifacts/templates-core-<VERSION>.tar.gz (+ .sha256 + install-manifest.json) for dockpipe install core.
# Archive layout: top-level directory "core/" (matches src/core category dirs; workflows/ excluded). Upload the three artifacts
# to the same HTTPS base URL you set as DOCKPIPE_INSTALL_BASE_URL (e.g. Cloudflare R2 public bucket).
# Override output dir with DOCKPIPE_ARTIFACTS_DIR (default: release/artifacts).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
V="$(tr -d ' \t\r\n' < VERSION)"
ART="${DOCKPIPE_ARTIFACTS_DIR:-release/artifacts}"
mkdir -p "$ART"
OUT="${ART}/templates-core-${V}.tar.gz"
tar czf "$OUT" -C src core --exclude='core/workflows'
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum "$OUT" | awk '{print $1}' > "${OUT}.sha256"
else
  shasum -a 256 "$OUT" | awk '{print $1}' > "${OUT}.sha256"
fi
HASH="$(tr -d ' \t\r\n' < "${OUT}.sha256")"
printf '%s\n' "{\"schema\":1,\"packages\":{\"core\":{\"version\":\"${V}\",\"tarball\":\"templates-core-${V}.tar.gz\",\"sha256\":\"${HASH}\"}}}" > "${ART}/install-manifest.json"
echo "Wrote ${OUT} ${OUT}.sha256 ${ART}/install-manifest.json"
