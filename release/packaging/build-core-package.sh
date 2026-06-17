#!/usr/bin/env bash
# Build dockpipe-core-<VERSION>.tar.gz from the compiled package store.
# Usage:
#   ./release/packaging/build-core-package.sh [version] [out-dir]
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

WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/dockpipe-core-build-XXXXXX")"
trap 'rm -rf "${WORKDIR}"' EXIT INT TERM

go run -trimpath -ldflags "-s -w -X main.Version=${VERSION}" ./src/cmd package compile core \
  --workdir "${WORKDIR}" \
  --from "${REPO_ROOT}/src/core" \
  --force

go run -trimpath -ldflags "-s -w -X main.Version=${VERSION}" ./src/cmd package build store \
  --workdir "${WORKDIR}" \
  --out "${OUT_DIR}" \
  --only core \
  --version "${VERSION}"

echo "Built dockpipe core package into ${OUT_DIR}"
