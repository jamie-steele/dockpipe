#!/usr/bin/env bash
# Collect a small set of candidate files for a DorkPipe edit request.
set -euo pipefail

ROOT="${1:?repo root required}"
ACTIVE_FILE="${2:-}"
MESSAGE="${3:-}"

cd "$ROOT"

emit() {
  local path="$1"
  [[ -z "$path" ]] && return 0
  [[ "$path" == /* ]] && path="${path#$ROOT/}"
  [[ -e "$path" ]] || return 0
  printf '%s\n' "$path"
}

emit "$ACTIVE_FILE"

git -C "$ROOT" status --short --untracked-files=no 2>/dev/null | awk '{print $2}' | while IFS= read -r path; do
  emit "$path"
done

printf '%s\n' "$MESSAGE" \
  | grep -Eo '[A-Za-z0-9_./-]+\.[A-Za-z0-9]+' \
  | while IFS= read -r hinted; do
      emit "$hinted"
    done

tokens="$(printf '%s\n' "$MESSAGE" | tr '[:upper:]' '[:lower:]' | grep -Eo '[a-z0-9_-]{4,}' | sort -u | sed -n '1,12p')"
if [[ -n "$tokens" ]]; then
  while IFS= read -r token; do
    [[ -z "$token" ]] && continue
    rg --files --hidden -g '!**/.git/**' -g '!**/node_modules/**' -g '!**/target/**' -g '!**/.dockpipe/**' -g '!**/.dorkpipe/**' "$ROOT" \
      | grep -i "$token" \
      | sed "s#^$ROOT/##" \
      | sed -n '1,4p'
  done <<<"$tokens"
fi \
  | while IFS= read -r path; do
      emit "$path"
    done
