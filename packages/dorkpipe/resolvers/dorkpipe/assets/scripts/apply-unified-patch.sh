#!/usr/bin/env bash
# Apply a validated unified diff patch to a repo working tree.
set -euo pipefail

PATCH_FILE="${1:?patch file required}"
ROOT="${2:?repo root required}"

git -C "$ROOT" apply --recount --whitespace=nowarn "$PATCH_FILE"
echo "Applied patch: $PATCH_FILE"
