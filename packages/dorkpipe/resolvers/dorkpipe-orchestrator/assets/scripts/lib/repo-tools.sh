#!/usr/bin/env bash
set -euo pipefail

dorkpipe_orchestrator_repo_root() {
  local root="${1:-${DOCKPIPE_WORKDIR:-$PWD}}"
  cd "$root" && pwd
}

dorkpipe_orchestrator_resolve_dorkpipe_bin() {
  local root="${1:-$(dorkpipe_orchestrator_repo_root)}"
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
