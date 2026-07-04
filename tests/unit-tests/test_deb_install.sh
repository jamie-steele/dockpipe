#!/usr/bin/env bash
# Integration test: install the .deb in a container and run dockpipe --help.
# Validates dpkg install + dockpipe --help (bundled assets are embedded in the binary).
# (fix for issue #1). Requires Docker and a built .deb; fails if either is missing.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEB_GLOB="${REPO_ROOT}/release/packaging/build/dockpipe_*_amd64.deb"
DEB="${DOCKPIPE_DEB_PATH:-}"
BUILD_ROOT="${REPO_ROOT}/release/packaging/build"

docker_mount_path() {
  local path="${1:?path}"
  case "${OSTYPE:-}" in
    msys*|cygwin*)
      if command -v cygpath >/dev/null 2>&1; then
        cygpath -w "$path"
        return 0
      fi
      ;;
  esac
  if [[ "$path" =~ ^/mnt/([a-zA-Z])/(.*)$ ]]; then
    local drive="${BASH_REMATCH[1]}"
    local rest="${BASH_REMATCH[2]//\//\\}"
    printf '%s\n' "${drive^^}:\\${rest}"
    return 0
  fi
  if [[ "$path" =~ ^/([a-zA-Z])/(.*)$ ]]; then
    local drive="${BASH_REMATCH[1]}"
    local rest="${BASH_REMATCH[2]//\//\\}"
    printf '%s\n' "${drive^^}:\\${rest}"
    return 0
  fi
  printf '%s\n' "$path"
}

rebuild_deb_from_package_dir() {
  local package_dir="${1:?package dir}"
  local tmp_dir out_deb package_mount tmp_mount
  tmp_dir="$(mktemp -d)"
  out_deb="${tmp_dir}/dockpipe-test.deb"
  package_mount="$(docker_mount_path "${package_dir}")"
  tmp_mount="$(docker_mount_path "${tmp_dir}")"
  docker run --rm \
    -v "${package_mount}:/in:ro" \
    -v "${tmp_mount}:/out" \
    debian:bookworm-slim \
    bash -lc 'set -euo pipefail; cp -a /in /tmp/pkg; find /tmp/pkg -type d -exec chmod 755 {} +; find /tmp/pkg -type f -exec chmod 644 {} +; chmod 755 /tmp/pkg/usr/bin/dockpipe; dpkg-deb --root-owner-group --build /tmp/pkg /out/dockpipe-test.deb >/dev/null'
  printf '%s\n' "$out_deb"
}

if ! command -v docker &>/dev/null; then
  echo "test_deb_install FAIL: Docker not found (required for this test)"
  exit 1
fi

if [[ -z "${DEB}" ]]; then
  DEB=$(echo $DEB_GLOB 2>/dev/null)
fi
if [[ ! -f "${DEB}" ]]; then
  package_dir="$(find "${BUILD_ROOT}" -maxdepth 1 -type d -name 'dockpipe_*_amd64' -exec test -f '{}/DEBIAN/control' ';' -print -quit 2>/dev/null || true)"
  if [[ -n "${package_dir}" ]]; then
    DEB="$(rebuild_deb_from_package_dir "${package_dir}")"
  else
    echo "test_deb_install FAIL: No .deb found at release/packaging/build/dockpipe_*_amd64.deb (run ./release/packaging/build-deb.sh first)"
    exit 1
  fi
fi

# Install .deb in container and run the installed binary directly. Stream the
# package over stdin so nested Docker clients (for example inside act on a
# Windows host) do not depend on bind-mount path translation.
docker_rc=0
if ! out=$(docker run --rm \
  -i \
  debian:bookworm-slim \
  bash -lc '
    set -euo pipefail
    cat > /tmp/dockpipe.deb
    dpkg -i --force-depends /tmp/dockpipe.deb >/tmp/dpkg.log 2>&1 || true
    if [[ ! -x /usr/bin/dockpipe ]]; then
      echo "installed binary missing: /usr/bin/dockpipe"
      cat /tmp/dpkg.log
      exit 1
    fi
    /usr/bin/dockpipe --help
  ' < "${DEB}" 2>&1); then
  docker_rc=$?
fi

if echo "$out" | grep -q "Usage\|dockpipe.*Run"; then
  echo "test_deb_install OK (.deb install path resolves, dockpipe --help works)"
else
  echo "test_deb_install FAIL: dockpipe --help did not succeed or output unexpected"
  echo "docker exit code: ${docker_rc}"
  echo "$out"
  exit 1
fi
