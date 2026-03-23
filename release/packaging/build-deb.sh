#!/usr/bin/env bash
# Build a .deb package. Run from repo root:
#   ./release/packaging/build-deb.sh [version] [deb-arch]
# deb-arch: amd64 (x86_64 Linux) or arm64 (aarch64 Linux). Default amd64.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

PKG_ROOT="${REPO_ROOT}/release/packaging"

_default_ver="$(tr -d ' \t\r\n' < "${REPO_ROOT}/VERSION" 2>/dev/null || true)"
[[ -z "${_default_ver}" ]] && _default_ver="0.6.0"
VERSION="${1:-${_default_ver}}"
DEB_ARCH="${2:-amd64}"
case "${DEB_ARCH}" in
  amd64 | arm64) ;;
  *)
    echo "usage: $0 [version] [amd64|arm64]" >&2
    exit 1
    ;;
esac
# Go uses the same GOARCH names for these Debian ports.
GOARCH="${DEB_ARCH}"
PACKAGE="dockpipe_${VERSION}_${DEB_ARCH}"
BUILD_DIR="${PKG_ROOT}/build/${PACKAGE}"

# Single binary in /usr/bin — templates/scripts/images are embedded (see embed.go).
mkdir -p "${BUILD_DIR}/usr/bin"
(
  cd "${REPO_ROOT}"
  GOOS=linux GOARCH="${GOARCH}" CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.Version=${VERSION}" -o "${BUILD_DIR}/usr/bin/dockpipe" ./src/cmd/dockpipe
)
chmod 755 "${BUILD_DIR}/usr/bin/dockpipe"

# Doc
mkdir -p "${BUILD_DIR}/usr/share/doc/dockpipe"
cp README.md LICENSE CONTRIBUTING.md "${BUILD_DIR}/usr/share/doc/dockpipe/"
cp -r docs "${BUILD_DIR}/usr/share/doc/dockpipe/" 2>/dev/null || true

# Debian control (version substituted)
mkdir -p "${BUILD_DIR}/DEBIAN"
sed -e "s/^Version: .*/Version: ${VERSION}/" -e "s/^Architecture: .*/Architecture: ${DEB_ARCH}/" "${PKG_ROOT}/control" > "${BUILD_DIR}/DEBIAN/control"

mkdir -p "${PKG_ROOT}/build"
dpkg-deb --root-owner-group --build "${BUILD_DIR}" "${PKG_ROOT}/build/${PACKAGE}.deb"
echo "Built: release/packaging/build/${PACKAGE}.deb"
rm -rf "${BUILD_DIR}"
