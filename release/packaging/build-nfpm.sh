#!/usr/bin/env bash
# Build Alpine (.apk), Fedora/RHEL (.rpm), and Arch Linux (.pkg.tar.zst) packages via nfpm.
# Run from repo root:
#   ./release/packaging/build-nfpm.sh [version] [dest-dir]
# dest-dir defaults to "release/artifacts" (same as CI release artifacts).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

_default_ver="$(tr -d ' \t\r\n' < "${REPO_ROOT}/VERSION" 2>/dev/null || true)"
[[ -z "${_default_ver}" ]] && _default_ver="0.0.0"
VERSION="${1:-${_default_ver}}"
DEST="${2:-release/artifacts}"
if [[ "${DEST}" == /* ]]; then
  OUT_DIR="${DEST}"
else
  OUT_DIR="${REPO_ROOT}/${DEST}"
fi
mkdir -p "${OUT_DIR}"

STAGE="${REPO_ROOT}/release/packaging/build/nfpm-stage"
rm -rf "${STAGE}"
mkdir -p "${STAGE}"

CORE_STAGE="${STAGE}/core-artifacts"
mkdir -p "${CORE_STAGE}"
bash "${REPO_ROOT}/release/packaging/build-core-package.sh" "${VERSION}" "${CORE_STAGE}"
CORE_TARBALL="$(find "${CORE_STAGE}" -maxdepth 1 -type f -name 'dockpipe-core-*.tar.gz' | sort | tail -n 1)"
if [[ -z "${CORE_TARBALL}" ]]; then
  echo "failed to build dockpipe-core tarball" >&2
  exit 1
fi
CORE_TARBALL_BASENAME="$(basename "${CORE_TARBALL}")"

# Pin nfpm for reproducible CI (bump when upgrading).
NFPM_VER="${NFPM_VERSION:-v2.41.0}"
NFPM=(go run "github.com/goreleaser/nfpm/v2/cmd/nfpm@${NFPM_VER}")
LDFLAGS="-s -w -X main.Version=${VERSION}"

for goarch in amd64 arm64; do
  BIN="${STAGE}/dockpipe-linux-${goarch}"
  GOOS=linux GOARCH="${goarch}" CGO_ENABLED=0 go build -trimpath -ldflags "${LDFLAGS}" -o "${BIN}" ./src/cmd

  sed -e "s|__VERSION__|${VERSION}|g" -e "s|__GOARCH__|${goarch}|g" -e "s|__BINARY__|${BIN}|g" \
    -e "s|__CORE_TARBALL__|${CORE_TARBALL}|g" \
    -e "s|__CORE_TARBALL_BASENAME__|${CORE_TARBALL_BASENAME}|g" \
    "${REPO_ROOT}/release/packaging/nfpm.yaml.in" > "${STAGE}/nfpm.yaml"

  "${NFPM[@]}" package -f "${STAGE}/nfpm.yaml" -p apk -t "${OUT_DIR}/dockpipe_${VERSION}_linux_${goarch}.apk"
  "${NFPM[@]}" package -f "${STAGE}/nfpm.yaml" -p rpm -t "${OUT_DIR}/dockpipe_${VERSION}_linux_${goarch}.rpm"
  "${NFPM[@]}" package -f "${STAGE}/nfpm.yaml" -p archlinux -t "${OUT_DIR}/dockpipe_${VERSION}_linux_${goarch}.pkg.tar.zst"
done

rm -rf "${STAGE}"
echo "Built nfpm packages into ${OUT_DIR}/ (apk, rpm, Arch)"
