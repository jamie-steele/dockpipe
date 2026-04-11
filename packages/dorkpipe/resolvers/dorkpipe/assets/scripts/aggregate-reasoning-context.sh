#!/usr/bin/env bash
# Merge node outputs under .dorkpipe/nodes into a single context block for downstream prompts.
set -euo pipefail
root="${DOCKPIPE_WORKDIR:-.}"
out="${1:-/dev/stdout}"
{
  echo "# DorkPipe aggregated context"
  find "$root/.dorkpipe/nodes" -maxdepth 1 -name '*.txt' -print 2>/dev/null | sort | while read -r f; do
    echo ""
    echo "## $(basename "$f" .txt)"
    cat "$f"
  done
} >"$out"
