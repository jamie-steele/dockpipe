#!/usr/bin/env bash
# Try git apply --check on a patch file (dry-run).
set -euo pipefail
patch="${1:?patch file}"
repo="${2:-.}"
git -C "$repo" apply --check "$patch"
