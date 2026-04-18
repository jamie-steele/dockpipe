#!/usr/bin/env bash
set -euo pipefail

dorkpipe_repo_root() {
  local root="${1:-${DOCKPIPE_WORKDIR:-$PWD}}"
  cd "$root" && pwd
}

dorkpipe_resolve_dockpipe_bin() {
  local root="${1:-$(dorkpipe_repo_root)}"
  local configured="${DOCKPIPE_BIN:-}"

  if [[ -n "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi
  if [[ -x "$root/src/bin/dockpipe" ]]; then
    printf '%s\n' "$root/src/bin/dockpipe"
    return 0
  fi
  command -v dockpipe 2>/dev/null || true
}

dorkpipe_resolve_dorkpipe_bin() {
  local root="${1:-$(dorkpipe_repo_root)}"
  local configured="${DORKPIPE_BIN:-}"

  if [[ -n "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi
  if [[ -x "$root/packages/dorkpipe/bin/dorkpipe" ]]; then
    printf '%s\n' "$root/packages/dorkpipe/bin/dorkpipe"
    return 0
  fi
  command -v dorkpipe 2>/dev/null || true
}
