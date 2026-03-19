#!/usr/bin/env bash
# Build a .deb package. Run from repo root: ./packaging/build-deb.sh [version]
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

VERSION="${1:-0.6.0}"
PACKAGE="dockpipe_${VERSION}_amd64"
BUILD_DIR="${REPO_ROOT}/packaging/build/${PACKAGE}"
DEST="${BUILD_DIR}/usr/lib/dockpipe"

rm -rf "${REPO_ROOT}/packaging/build"
mkdir -p "${DEST}/bin"

# Core layout (same as repo so DOCKPIPE_REPO_ROOT works)
cp -r lib scripts images templates "${DEST}/"
echo "${VERSION}" > "${DEST}/version"
chmod 755 "${DEST}/lib/"*.sh
chmod 755 "${DEST}/scripts/"*.sh 2>/dev/null || true
find "${DEST}/templates" -name "*.sh" -exec chmod 755 {} \; 2>/dev/null || true

# Go binary (linux/amd64; adjust GOARCH for other targets)
(
  cd "${REPO_ROOT}"
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "${DEST}/bin/dockpipe" ./cmd/dockpipe
)
chmod 755 "${DEST}/bin/dockpipe"

# Symlink from PATH
mkdir -p "${BUILD_DIR}/usr/bin"
ln -s ../lib/dockpipe/bin/dockpipe "${BUILD_DIR}/usr/bin/dockpipe"

# Doc
mkdir -p "${BUILD_DIR}/usr/share/doc/dockpipe"
cp README.md LICENSE CONTRIBUTING.md "${BUILD_DIR}/usr/share/doc/dockpipe/"
cp -r docs "${BUILD_DIR}/usr/share/doc/dockpipe/" 2>/dev/null || true

# Debian control (version substituted)
mkdir -p "${BUILD_DIR}/DEBIAN"
sed "s/^Version: .*/Version: ${VERSION}/" packaging/control > "${BUILD_DIR}/DEBIAN/control"

dpkg-deb --root-owner-group --build "${BUILD_DIR}" "${REPO_ROOT}/packaging/build/${PACKAGE}.deb"
echo "Built: packaging/build/${PACKAGE}.deb"
rm -rf "${BUILD_DIR}"
