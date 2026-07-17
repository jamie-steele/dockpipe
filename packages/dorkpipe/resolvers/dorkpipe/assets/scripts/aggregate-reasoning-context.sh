#!/usr/bin/env bash
# Merge package-scoped node outputs into a single context block for downstream prompts.
set -euo pipefail
DORKPIPE_NODES_DIR="$(dockpipe scope --package dorkpipe nodes)"
out="${1:-/dev/stdout}"
{
  echo "# DorkPipe aggregated context"
  find "$DORKPIPE_NODES_DIR" -maxdepth 1 -name '*.txt' -print 2>/dev/null | sort | while read -r f; do
    echo ""
    echo "## $(basename "$f" .txt)"
    cat "$f"
  done
} >"$out"
