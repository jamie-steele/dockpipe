#!/usr/bin/env bash
# Pipeon helper resolution should prefer the repo-local dockpipe binary.
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"

# shellcheck source=/dev/null
source "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh"

actual="$(DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_sdk require dockpipe-bin' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
expected="$(DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_resolve_dockpipe_bin "$2"' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh" "$ROOT")"

if [[ "$actual" != "$expected" ]]; then
  echo "test_repo_tools: expected $expected, got $actual" >&2
  exit 1
fi

echo "test_repo_tools OK"
