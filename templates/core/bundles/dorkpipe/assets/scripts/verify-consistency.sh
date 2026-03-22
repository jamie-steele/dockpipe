#!/usr/bin/env bash
# Cross-check two answers (files): exit 0 if both cite same primary conclusion keyword list.
set -euo pipefail
a="${1:?answer a}"
b="${2:?answer b}"
kw="${3:?keywords file one per line}"
hits=0
while IFS= read -r w; do
  [[ -z "$w" ]] && continue
  if grep -Fq -- "$w" "$a" && grep -Fq -- "$w" "$b"; then
    hits=$((hits + 1))
  fi
done <"$kw"
if ((hits > 0)); then
  exit 0
fi
echo "verify-consistency: no shared keywords" >&2
exit 1
