#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
src_dir="${script_dir}/dockpipe-guest-agent-src"
out_file="${script_dir}/dockpipe-guest-agent.exe"
target_arch="${DOCKPIPE_VM_GUEST_AGENT_GOARCH:-amd64}"

[[ -d "${src_dir}" ]] || {
  echo "dockpipe guest agent source not found: ${src_dir}" >&2
  exit 1
}

export GOOS=windows
export GOARCH="${target_arch}"
export CGO_ENABLED=0
export GO111MODULE=off

(
  cd "${src_dir}"
  go build -trimpath -ldflags="-s -w" -o "${out_file}" .
)

# Source-build hooks produce the shipped artifact; the source tree does not need
# to ride along inside compiled core tarballs.
rm -rf "${src_dir}"
