#!/usr/bin/env bash
# Combine verifier notes + ranked evidence + aggregated node context for final synthesis.
set -euo pipefail
wd="${DOCKPIPE_WORKDIR:?DOCKPIPE_WORKDIR is required}"
evidence="${1:-}"
{
  echo "# Final synthesis pack"
  if [[ -f "$wd/bin/.dockpipe/packages/dorkpipe/run.json" ]]; then
    echo "## run.json (excerpt)"
    head -c 8000 "$wd/bin/.dockpipe/packages/dorkpipe/run.json" || true
    echo ""
  fi
  if [[ -n "$evidence" && -f "$evidence" ]]; then
    echo "## evidence"
    cat "$evidence"
  fi
  if [[ -d "$wd/bin/.dockpipe/packages/dorkpipe/nodes" ]]; then
    echo "## node outputs"
    find "$wd/bin/.dockpipe/packages/dorkpipe/nodes" -maxdepth 1 -name '*.txt' | sort | while read -r f; do
      echo "### $(basename "$f")"
      cat "$f"
    done
  fi
} | tee "${2:-/dev/stdout}"
