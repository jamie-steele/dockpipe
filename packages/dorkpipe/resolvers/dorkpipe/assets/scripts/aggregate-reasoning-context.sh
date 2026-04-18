#!/usr/bin/env bash
# Merge node outputs under bin/.dockpipe/packages/dorkpipe/nodes into a single context block for downstream prompts.
set -euo pipefail
root="${DOCKPIPE_WORKDIR:?DOCKPIPE_WORKDIR is required}"
out="${1:-/dev/stdout}"
{
  echo "# DorkPipe aggregated context"
  find "$root/bin/.dockpipe/packages/dorkpipe/nodes" -maxdepth 1 -name '*.txt' -print 2>/dev/null | sort | while read -r f; do
    echo ""
    echo "## $(basename "$f" .txt)"
    cat "$f"
  done
} >"$out"
