#!/usr/bin/env bash
# Build Alpine (.apk), Fedora/RHEL (.rpm), and Arch Linux (.pkg.tar.zst) packages via nfpm.
# Run from repo root:
#   ./release/packaging/build-nfpm.sh [version] [dest-dir]
# dest-dir defaults to "dist" (same as CI release artifacts).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

_default_ver="$(tr -d ' \t\r\n' < "${REPO_ROOT}/VERSION" 2>/dev/null || true)"
[[ -z "${_default_ver}" ]] && _default_ver="0.0.0"
VERSION="${1:-${_default_ver}}"
DEST="${2:-dist}"
if [[ "${DEST}" == /* ]]; then
  OUT_DIR="${DEST}"
else
  OUT_DIR="${REPO_ROOT}/${DEST}"
fi
mkdir -p "${OUT_DIR}"

STAGE="${REPO_ROOT}/release/packaging/build/nfpm-stage"
rm -rf "${STAGE}"
mkdir -p "${STAGE}"

# Pin nfpm for reproducible CI (bump when upgrading).
NFPM_VER="${NFPM_VERSION:-v2.41.0}"
NFPM=(go run "github.com/goreleaser/nfpm/v2/cmd/nfpm@${NFPM_VER}")
LDFLAGS="-s -w -X main.Version=${VERSION}"

for goarch in amd64 arm64; do
  BIN="${STAGE}/dockpipe-linux-${goarch}"
  GOOS=linux GOARCH="${goarch}" CGO_ENABLED=0 go build -trimpath -ldflags "${LDFLAGS}" -o "${BIN}" ./src/cmd/dockpipe

  sed -e "s|__VERSION__|${VERSION}|g" -e "s|__GOARCH__|${goarch}|g" -e "s|__BINARY__|${BIN}|g" \
    "${REPO_ROOT}/release/packaging/nfpm.yaml.in" > "${STAGE}/nfpm.yaml"

  "${NFPM[@]}" package -f "${STAGE}/nfpm.yaml" -p apk -t "${OUT_DIR}/dockpipe_${VERSION}_linux_${goarch}.apk"
  "${NFPM[@]}" package -f "${STAGE}/nfpm.yaml" -p rpm -t "${OUT_DIR}/dockpipe_${VERSION}_linux_${goarch}.rpm"
  "${NFPM[@]}" package -f "${STAGE}/nfpm.yaml" -p archlinux -t "${OUT_DIR}/dockpipe_${VERSION}_linux_${goarch}.pkg.tar.zst"
done

rm -rf "${STAGE}"
echo "Built nfpm packages into ${OUT_DIR}/ (apk, rpm, Arch)"
