#!/usr/bin/env bash
set -euo pipefail

SOURCE_DIR="${DOCKPIPE_COMPILE_SOURCE_DIR:?DOCKPIPE_COMPILE_SOURCE_DIR is required}"
STAGING_DIR="${DOCKPIPE_COMPILE_STAGING_DIR:?DOCKPIPE_COMPILE_STAGING_DIR is required}"
COMPILE_WORKDIR="${DOCKPIPE_COMPILE_WORKDIR:?DOCKPIPE_COMPILE_WORKDIR is required}"

PACKAGE_ROOT="$(cd "${SOURCE_DIR}/../.." && pwd)"
REPO_ROOT="$(cd "${PACKAGE_ROOT}/../.." && pwd)"
OUT_ROOT="${STAGING_DIR}/assets/tooling/bin"
BUILD_ROOT="${COMPILE_WORKDIR}/bin/.dockpipe/build/dorkpipe-consumer-artifacts"
VERSION_FILE="${REPO_ROOT}/VERSION"

host_os="$(go env GOHOSTOS)"
host_arch="$(go env GOHOSTARCH)"
linux_arch="${DORKPIPE_STACK_GOARCH:-${host_arch}}"
version="0.0.0"
if [[ -f "${VERSION_FILE}" ]]; then
  version="$(tr -d ' \t\r\n' < "${VERSION_FILE}")"
fi
ldflags="-s -w -X main.Version=${version}"

mkdir -p "${OUT_ROOT}" "${BUILD_ROOT}/go-cache" "${BUILD_ROOT}/go-tmp"
export GOCACHE="${GOCACHE:-${BUILD_ROOT}/go-cache}"
export GOTMPDIR="${GOTMPDIR:-${BUILD_ROOT}/go-tmp}"

build_go_binary() {
  local target_os="$1"
  local target_arch="$2"
  local module_dir="$3"
  local pkg="$4"
  local output="$5"

  mkdir -p "$(dirname "${output}")"
  GOOS="${target_os}" GOARCH="${target_arch}" CGO_ENABLED=0 \
    go build -C "${module_dir}" -trimpath -ldflags "${ldflags}" -o "${output}" "${pkg}"
}

host_dir="${OUT_ROOT}/${host_os}"
linux_dir="${OUT_ROOT}/linux"
host_exe=""
if [[ "${host_os}" == "windows" ]]; then
  host_exe=".exe"
fi

build_go_binary "${host_os}" "${host_arch}" "${PACKAGE_ROOT}/lib" ./cmd/dorkpipe "${host_dir}/dorkpipe${host_exe}"
build_go_binary "${host_os}" "${host_arch}" "${PACKAGE_ROOT}/mcp" ./cmd/mcpd "${host_dir}/mcpd${host_exe}"
build_go_binary "${host_os}" "${host_arch}" "${PACKAGE_ROOT}/lib" ./cmd/skills-render "${host_dir}/skills-render${host_exe}"
build_go_binary "${host_os}" "${host_arch}" "${PACKAGE_ROOT}/lib" ./cmd/orchestrate-helper "${host_dir}/orchestrate-helper${host_exe}"

build_go_binary linux "${linux_arch}" "${REPO_ROOT}" ./src/cmd "${linux_dir}/dockpipe"
build_go_binary linux "${linux_arch}" "${PACKAGE_ROOT}/lib" ./cmd/dorkpipe "${linux_dir}/dorkpipe"
build_go_binary linux "${linux_arch}" "${PACKAGE_ROOT}/mcp" ./cmd/mcpd "${linux_dir}/mcpd"
