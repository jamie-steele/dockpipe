#!/usr/bin/env bash
# Merge package-scoped node outputs into a single context block for downstream prompts.
set -euo pipefail
root="${DOCKPIPE_WORKDIR:?DOCKPIPE_WORKDIR is required}"
if [[ -n "${DOCKPIPE_SDK_SH:-}" && -f "$DOCKPIPE_SDK_SH" ]]; then
  # shellcheck source=/dev/null
  source "$DOCKPIPE_SDK_SH"
  dockpipe_sdk_refresh "$root"
else
  eval "$(dockpipe sdk --workdir "$root")"
fi
DORKPIPE_NODES_DIR="$(dockpipe_sdk scope --package dorkpipe nodes)"
out="${1:-/dev/stdout}"
{
  echo "# DorkPipe aggregated context"
  find "$DORKPIPE_NODES_DIR" -maxdepth 1 -name '*.txt' -print 2>/dev/null | sort | while read -r f; do
    echo ""
    echo "## $(basename "$f" .txt)"
    cat "$f"
  done
} >"$out"
