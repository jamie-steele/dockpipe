#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:?repo root required}"
TARGET="${2:-}"
ACTIVE_FILE="${3:-}"
SELECTION="${4:-}"

cd "$ROOT"

pick_symbol() {
  local value
  value="$(printf '%s' "$TARGET" | tr '\n' ' ' | xargs 2>/dev/null || true)"
  if [[ -n "$value" ]]; then
    printf '%s\n' "$value"
    return 0
  fi
  value="$(printf '%s' "$SELECTION" | tr '\n' ' ' | sed -E 's/^[[:space:]]+|[[:space:]]+$//g' | awk '{print $1}' || true)"
  if [[ -n "$value" ]]; then
    printf '%s\n' "$value"
    return 0
  fi
  return 1
}

SYMBOL="$(pick_symbol || true)"

if [[ -z "$SYMBOL" ]]; then
  cat <<'EOF'
No symbol could be inferred.

Try:
- /references MyType
- select the identifier first
EOF
  exit 0
fi

echo "Target symbol: $SYMBOL"
[[ -n "$ACTIVE_FILE" ]] && echo "Active file: $ACTIVE_FILE"
echo

echo "References:"
rg -n --hidden -g '!**/.git/**' -g '!**/node_modules/**' -g '!**/target/**' -g '!**/.dockpipe/**' -g '!**/.dorkpipe/**' \
  "\b${SYMBOL}\b" . | sed -n '1,60p' || true
