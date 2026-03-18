#!/usr/bin/env bash
# Reads JSON config from stdin; exits 0 if valid.
set -euo pipefail
config=$(cat)
if echo "$config" | jq -e . >/dev/null 2>&1; then
  echo "[validate] Config is valid JSON"
  exit 0
else
  echo "[validate] Invalid JSON" >&2
  exit 1
fi
