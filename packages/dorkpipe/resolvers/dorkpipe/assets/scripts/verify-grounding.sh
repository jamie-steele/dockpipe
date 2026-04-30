#!/usr/bin/env bash
# Stub: check that key claims in $1 appear in corpus file $2 (grep -F). Exit 1 if missing.
set -euo pipefail
claims="${1:?claims file}"
corpus="${2:?corpus file}"
missing=0
while IFS= read -r line; do
  line=$(echo "$line" | sed '/^\s*$/d' | head -1)
  [[ -z "$line" ]] && continue
  if ! grep -Fq -- "$line" "$corpus" 2>/dev/null; then
    echo "verify-grounding: missing claim: $line" >&2
    missing=1
  fi
done <"$claims"
exit "$missing"
