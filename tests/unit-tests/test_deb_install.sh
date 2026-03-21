#!/usr/bin/env bash
# Integration test: install the .deb in a container and run dockpipe --help.
# Validates dpkg install + dockpipe --help (bundled assets are embedded in the binary).
# (fix for issue #1). Requires Docker and a built .deb; fails if either is missing.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEB_GLOB="${REPO_ROOT}/packaging/build/dockpipe_*_amd64.deb"
DEB="${DOCKPIPE_DEB_PATH:-}"

if ! command -v docker &>/dev/null; then
  echo "test_deb_install FAIL: Docker not found (required for this test)"
  exit 1
fi

if [[ -z "${DEB}" ]]; then
  DEB=$(echo $DEB_GLOB 2>/dev/null)
fi
if [[ ! -f "${DEB}" ]]; then
  echo "test_deb_install FAIL: No .deb found at packaging/build/dockpipe_*_amd64.deb (run ./packaging/build-deb.sh first)"
  exit 1
fi

# Install .deb in container and run dockpipe --help. --force-depends skips docker.io.
out=$(docker run --rm \
  -v "${DEB}:/dockpipe.deb:ro" \
  debian:bookworm-slim \
  bash -c 'dpkg -i --force-depends /dockpipe.deb 2>/dev/null; dockpipe --help' 2>&1)

if echo "$out" | grep -q "Usage\|dockpipe.*Run"; then
  echo "test_deb_install OK (.deb install path resolves, dockpipe --help works)"
else
  echo "test_deb_install FAIL: dockpipe --help did not succeed or output unexpected"
  echo "$out"
  exit 1
fi
