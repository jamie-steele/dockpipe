#!/usr/bin/env bash
# Package pgvector / search output into a single markdown chunk list for prompts.
set -euo pipefail
in="${1:?tsv or text from retrieval node}"
out="${2:-/dev/stdout}"
{
  echo "# Retrieval bundle"
  echo '```'
  cat "$in"
  echo '```'
} >"$out"
