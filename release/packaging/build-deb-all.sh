#!/usr/bin/env bash
# Build both amd64 and arm64 .deb packages. Run from repo root:
#   ./release/packaging/build-deb-all.sh [version]
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
_default_ver="$(tr -d ' \t\r\n' < "${ROOT}/VERSION" 2>/dev/null || true)"
[[ -z "${_default_ver}" ]] && _default_ver="0.6.0"
VERSION="${1:-${_default_ver}}"
"${ROOT}/release/packaging/build-deb.sh" "${VERSION}" amd64
"${ROOT}/release/packaging/build-deb.sh" "${VERSION}" arm64
echo "Built: release/packaging/build/dockpipe_${VERSION}_amd64.deb"
echo "Built: release/packaging/build/dockpipe_${VERSION}_arm64.deb"
