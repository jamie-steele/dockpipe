#!/usr/bin/env bash
# Build a .deb package. Run from repo root: ./packaging/build-deb.sh [version]
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

VERSION="${1:-0.3.0}"
PACKAGE="dockpipe_${VERSION}_all"
BUILD_DIR="${REPO_ROOT}/packaging/build/${PACKAGE}"
DEST="${BUILD_DIR}/usr/lib/dockpipe"

rm -rf "${REPO_ROOT}/packaging/build"
mkdir -p "${DEST}"

# Copy core files (same layout as repo so DOCKPIPE_REPO_ROOT works)
cp -r bin lib images examples "${DEST}/"
chmod 755 "${DEST}/bin/dockpipe"
chmod 755 "${DEST}/lib/"*.sh
chmod 755 "${DEST}/examples/actions/"*.sh
chmod 755 "${DEST}/examples/claude-worktree/"*.sh 2>/dev/null || true

# Symlink from PATH
mkdir -p "${BUILD_DIR}/usr/bin"
ln -s ../lib/dockpipe/bin/dockpipe "${BUILD_DIR}/usr/bin/dockpipe"

# Doc
mkdir -p "${BUILD_DIR}/usr/share/doc/dockpipe"
cp README.md LICENSE "${BUILD_DIR}/usr/share/doc/dockpipe/"
cp -r docs "${BUILD_DIR}/usr/share/doc/dockpipe/" 2>/dev/null || true

# Debian control (version substituted)
mkdir -p "${BUILD_DIR}/DEBIAN"
sed "s/^Version: .*/Version: ${VERSION}/" packaging/control > "${BUILD_DIR}/DEBIAN/control"

dpkg-deb --root-owner-group --build "${BUILD_DIR}" "${REPO_ROOT}/packaging/build/${PACKAGE}.deb"
echo "Built: packaging/build/${PACKAGE}.deb"
rm -rf "${BUILD_DIR}"
