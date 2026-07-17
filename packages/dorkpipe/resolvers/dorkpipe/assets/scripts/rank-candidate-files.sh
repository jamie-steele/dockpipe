#!/usr/bin/env bash
# Rank paths by simple heuristics (mtime, size); stdin = one path per line.
set -euo pipefail
while IFS= read -r p; do
  [[ -z "$p" ]] && continue
  [[ -e "$p" ]] || continue
  sz=$(stat -c%s "$p" 2>/dev/null || stat -f%z "$p" 2>/dev/null || echo 0)
  echo "$sz $p"
done | sort -nr | awk '{print $2}'
