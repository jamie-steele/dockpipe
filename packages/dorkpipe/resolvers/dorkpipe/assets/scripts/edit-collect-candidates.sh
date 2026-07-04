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

list_repo_files() {
  if command -v rg >/dev/null 2>&1; then
    rg --files --hidden -g '!**/.git/**' -g '!**/node_modules/**' -g '!**/target/**' -g '!**/.dockpipe/**' -g '!**/.dorkpipe/**' "$ROOT"
    return 0
  fi
  find "$ROOT" \
    -path "$ROOT/.git" -prune -o \
    -path "$ROOT/node_modules" -prune -o \
    -path "$ROOT/target" -prune -o \
    -path "$ROOT/.dockpipe" -prune -o \
    -path "$ROOT/.dorkpipe" -prune -o \
    -type f -print
}

emit "$ACTIVE_FILE"

lower_message="$(printf '%s\n' "$MESSAGE" | tr '[:upper:]' '[:lower:]')"

git -C "$ROOT" status --short --untracked-files=no 2>/dev/null | awk '{print $2}' | while IFS= read -r path; do
  emit "$path"
done

printf '%s\n' "$MESSAGE" \
  | grep -Eo '[A-Za-z0-9_./-]+\.[A-Za-z0-9]+' || true \
  | while IFS= read -r hinted; do
      emit "$hinted"
    done

tokens="$(
  printf '%s\n' "$MESSAGE" \
    | tr '[:upper:]' '[:lower:]' \
    | grep -Eo '[a-z0-9_-]{4,}' || true
)"
tokens="$(printf '%s\n' "$tokens" | sort -u | sed -n '1,12p')"
if [[ -n "$tokens" ]]; then
  while IFS= read -r token; do
    [[ -z "$token" ]] && continue
    list_repo_files \
      | { grep -i "$token" || true; } \
      | sed "s#^$ROOT/##" \
      | sed -n '1,4p'
  done <<<"$tokens"
fi \
  | while IFS= read -r path; do
      emit "$path"
    done

if printf '%s' "$lower_message" | grep -q 'package'; then
  list_repo_files \
    | { grep -E '(^|/)(package\.yml|config\.yml)$' || true; } \
    | sed -n '1,8p' \
    | while IFS= read -r path; do
        emit "$path"
      done
fi
