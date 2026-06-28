#!/usr/bin/env bash
# Combine verifier notes + ranked evidence + aggregated node context for final synthesis.
set -euo pipefail
wd="${DOCKPIPE_WORKDIR:?DOCKPIPE_WORKDIR is required}"
if [[ -n "${DOCKPIPE_SDK_SH:-}" && -f "$DOCKPIPE_SDK_SH" ]]; then
  # shellcheck source=/dev/null
  source "$DOCKPIPE_SDK_SH"
  dockpipe_sdk_refresh "$wd"
else
  eval "$(dockpipe sdk --workdir "$wd")"
fi
DORKPIPE_STATE_DIR="$(dockpipe_sdk scope --package dorkpipe .)"
evidence="${1:-}"
{
  echo "# Final synthesis pack"
  if [[ -f "$DORKPIPE_STATE_DIR/run.json" ]]; then
    echo "## run.json (excerpt)"
    head -c 8000 "$DORKPIPE_STATE_DIR/run.json" || true
    echo ""
  fi
  if [[ -n "$evidence" && -f "$evidence" ]]; then
    echo "## evidence"
    cat "$evidence"
  fi
  if [[ -d "$DORKPIPE_STATE_DIR/nodes" ]]; then
    echo "## node outputs"
    find "$DORKPIPE_STATE_DIR/nodes" -maxdepth 1 -name '*.txt' | sort | while read -r f; do
      echo "### $(basename "$f")"
      cat "$f"
    done
  fi
} | tee "${2:-/dev/stdout}"
