#!/usr/bin/env bash
# Run lightweight post-apply checks for changed files.
set -euo pipefail

ROOT="${1:?repo root required}"
shift

cd "$ROOT"

resolve_dockpipe_bin() {
  if [[ -x "$ROOT/src/bin/dockpipe" ]]; then
    printf '%s\n' "$ROOT/src/bin/dockpipe"
    return 0
  fi
  command -v dockpipe 2>/dev/null || true
}

DOCKPIPE_VALIDATE_BIN="$(resolve_dockpipe_bin)"

ran=0
for rel in "$@"; do
  [[ -z "$rel" ]] && continue
  [[ -e "$rel" ]] || continue
  case "$rel" in
    *.js|*.cjs|*.mjs)
      echo "[node --check] $rel"
      node --check "$rel"
      ran=1
      ;;
    *.json)
      if command -v jq >/dev/null 2>&1; then
        echo "[jq] $rel"
        jq empty "$rel" >/dev/null
        ran=1
      fi
      ;;
    *.yml|*.yaml)
      if [[ -n "$DOCKPIPE_VALIDATE_BIN" && "$(basename "$rel")" == "config.yml" ]]; then
        echo "[dockpipe workflow validate] $rel"
        "$DOCKPIPE_VALIDATE_BIN" workflow validate "$rel" >/dev/null
        ran=1
      fi
      ;;
  esac
done

if [[ "$ran" -eq 0 ]]; then
  echo "No targeted post-apply validators matched the changed files."
fi
