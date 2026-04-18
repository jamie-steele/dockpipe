#!/usr/bin/env bash
# Run lightweight post-apply checks for changed files.
set -euo pipefail

ROOT="${1:?repo root required}"
shift

cd "$ROOT"

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
      if [[ "$(basename "$rel")" == "config.yml" ]]; then
        echo "[dockpipe workflow validate] $rel"
        dockpipe workflow validate "$rel" >/dev/null
        ran=1
      fi
      ;;
  esac
done

if [[ "$ran" -eq 0 ]]; then
  echo "No targeted post-apply validators matched the changed files."
fi
