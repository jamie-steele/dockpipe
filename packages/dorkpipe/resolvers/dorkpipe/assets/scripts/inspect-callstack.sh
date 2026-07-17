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
  if [[ -n "$ACTIVE_FILE" && -f "$ACTIVE_FILE" ]]; then
    value="$(
      grep -nE '^(func|function|class|def)[[:space:]]+' "$ACTIVE_FILE" 2>/dev/null \
        | head -n 1 \
        | sed -E 's/^[0-9]+:[[:space:]]*(func|function|class|def)[[:space:]]+//; s/\(.*$//; s/[[:space:]]+$//'
    )"
    if [[ -n "$value" ]]; then
      printf '%s\n' "$value"
      return 0
    fi
  fi
  return 1
}

SYMBOL="$(pick_symbol || true)"

if [[ -z "$SYMBOL" ]]; then
  cat <<'EOF'
No symbol or stack target could be inferred.

Try:
- /callstack myFunction
- select a symbol before asking
- keep an active file open with the target function/class
EOF
  exit 0
fi

echo "Target symbol: $SYMBOL"
[[ -n "$ACTIVE_FILE" ]] && echo "Active file: $ACTIVE_FILE"
echo

echo "Definitions:"
if command -v rg >/dev/null 2>&1; then
  rg -n --hidden -g '!**/.git/**' -g '!**/node_modules/**' -g '!**/target/**' -g '!**/.dockpipe/**' -g '!**/.dorkpipe/**' \
    "^(func|function|class|def)[[:space:]]+${SYMBOL}\b|${SYMBOL}[[:space:]]*[:=][[:space:]]*(func|\()" . \
    | sed -n '1,20p' || true
else
  grep -RInE "^(func|function|class|def)[[:space:]]+${SYMBOL}\b|${SYMBOL}[[:space:]]*[:=][[:space:]]*(func|\()" . 2>/dev/null \
    | sed -n '1,20p' || true
fi
echo

echo "Likely callers / references:"
if command -v rg >/dev/null 2>&1; then
  rg -n --hidden -g '!**/.git/**' -g '!**/node_modules/**' -g '!**/target/**' -g '!**/.dockpipe/**' -g '!**/.dorkpipe/**' \
    "\b${SYMBOL}\s*\(" . \
    | sed -n '1,30p' || true
else
  grep -RInE "\b${SYMBOL}[[:space:]]*\(" . 2>/dev/null | sed -n '1,30p' || true
fi
echo

echo "Related symbol mentions:"
if command -v rg >/dev/null 2>&1; then
  rg -n --hidden -g '!**/.git/**' -g '!**/node_modules/**' -g '!**/target/**' -g '!**/.dockpipe/**' -g '!**/.dorkpipe/**' \
    "\b${SYMBOL}\b" . \
    | sed -n '1,40p' || true
else
  grep -RInE "\b${SYMBOL}\b" . 2>/dev/null | sed -n '1,40p' || true
fi
