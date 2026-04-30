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
      rg -n '^\s*(func|type|class|interface|const|var)\s+' "$ACTIVE_FILE" 2>/dev/null \
        | sed -n '1p' \
        | sed -E 's/^[0-9]+:\s*(func|type|class|interface|const|var)\s+//; s/\(.*$//; s/[[:space:]]+$//'
    )"
    if [[ -n "$value" ]]; then
      printf '%s\n' "$value"
      return 0
    fi
  fi
  return 1
}

show_excerpt() {
  local file="$1"
  local line="$2"
  python3 - "$file" "$line" <<'PY'
import sys
path = sys.argv[1]
line_no = int(sys.argv[2])
start = max(1, line_no - 3)
end = line_no + 8
with open(path, "r", encoding="utf-8", errors="replace") as fh:
    lines = fh.readlines()
for idx in range(start, min(end, len(lines)) + 1):
    text = lines[idx - 1].rstrip("\n")
    print(f"{idx}:{text}")
PY
}

SYMBOL="$(pick_symbol || true)"

if [[ -z "$SYMBOL" ]]; then
  cat <<'EOF'
No symbol could be inferred.

Try:
- /symbol MyType
- select the symbol first
- keep an active file open near the definition
EOF
  exit 0
fi

echo "Target symbol: $SYMBOL"
[[ -n "$ACTIVE_FILE" ]] && echo "Active file: $ACTIVE_FILE"
echo

pattern="(^|[[:space:][:punct:]])(func[[:space:]]+([^)]+[[:space:]])?${SYMBOL}\b|type[[:space:]]+${SYMBOL}\b|class[[:space:]]+${SYMBOL}\b|interface[[:space:]]+${SYMBOL}\b|const[[:space:]]+${SYMBOL}\b|var[[:space:]]+${SYMBOL}\b)"
matches="$(rg -n --hidden -g '!**/.git/**' -g '!**/node_modules/**' -g '!**/target/**' -g '!**/.dockpipe/**' -g '!**/.dorkpipe/**' "$pattern" . | sed -n '1,8p' || true)"

if [[ -z "$matches" ]]; then
  echo "No likely definitions found."
  exit 0
fi

echo "Likely definitions:"
printf '%s\n' "$matches"
echo

first="$(printf '%s\n' "$matches" | sed -n '1p')"
file="$(printf '%s' "$first" | cut -d: -f1)"
line="$(printf '%s' "$first" | cut -d: -f2)"

if [[ -n "$file" && -n "$line" && -f "$file" ]]; then
  echo "Definition excerpt:"
  show_excerpt "$file" "$line"
fi
