#!/usr/bin/env bash
# Line Jaccard between two text files (unique non-empty lines).
set -euo pipefail
if (($# < 2)); then
  echo "usage: $0 <file1> <file2>" >&2
  exit 2
fi
t1=$(mktemp)
t2=$(mktemp)
trap 'rm -f "$t1" "$t2"' EXIT
grep -v '^[[:space:]]*$' "$1" | sort -u >"$t1"
grep -v '^[[:space:]]*$' "$2" | sort -u >"$t2"
i=$(comm -12 "$t1" "$t2" | wc -l | awk '{print $1}')
n1=$(wc -l <"$t1" | awk '{print $1}')
n2=$(wc -l <"$t2" | awk '{print $1}')
u=$((n1 + n2 - i))
if ((u <= 0)); then
  echo "1.0000"
  exit 0
fi
awk -v i="$i" -v u="$u" 'BEGIN { printf "%.4f\n", i/u }'
