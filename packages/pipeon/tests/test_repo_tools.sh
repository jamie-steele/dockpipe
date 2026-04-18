#!/usr/bin/env bash
# Pipeon helper resolution should prefer the repo-local dockpipe binary.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# shellcheck source=../resolvers/pipeon/assets/scripts/lib/repo-tools.sh
source "$ROOT/packages/pipeon/resolvers/pipeon/assets/scripts/lib/repo-tools.sh"

expected="$ROOT/src/bin/dockpipe"
actual="$(DOCKPIPE_WORKDIR="$ROOT" pipeon_resolve_dockpipe_bin "$ROOT")"

if [[ "$actual" != "$expected" ]]; then
  echo "test_repo_tools: expected $expected, got $actual" >&2
  exit 1
fi

echo "test_repo_tools OK"
