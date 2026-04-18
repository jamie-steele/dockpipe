#!/usr/bin/env bash
# DorkPipe helper resolution should prefer repo-local binaries.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# shellcheck source=../resolvers/dorkpipe/assets/scripts/lib/repo-tools.sh
source "$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts/lib/repo-tools.sh"
# shellcheck source=../resolvers/dorkpipe-orchestrator/assets/scripts/lib/repo-tools.sh
source "$ROOT/packages/dorkpipe/resolvers/dorkpipe-orchestrator/assets/scripts/lib/repo-tools.sh"

expected_dockpipe="$ROOT/src/bin/dockpipe"
actual_dockpipe="$(DOCKPIPE_WORKDIR="$ROOT" dorkpipe_resolve_dockpipe_bin "$ROOT")"
if [[ "$actual_dockpipe" != "$expected_dockpipe" ]]; then
  echo "test_repo_tools: expected dockpipe $expected_dockpipe, got $actual_dockpipe" >&2
  exit 1
fi

expected_dorkpipe="$ROOT/packages/dorkpipe/bin/dorkpipe"
actual_dorkpipe="$(DOCKPIPE_WORKDIR="$ROOT" dorkpipe_resolve_dorkpipe_bin "$ROOT")"
if [[ "$actual_dorkpipe" != "$expected_dorkpipe" ]]; then
  echo "test_repo_tools: expected dorkpipe $expected_dorkpipe, got $actual_dorkpipe" >&2
  exit 1
fi

actual_orch="$(DOCKPIPE_WORKDIR="$ROOT" dorkpipe_orchestrator_resolve_dorkpipe_bin "$ROOT")"
if [[ "$actual_orch" != "$expected_dorkpipe" ]]; then
  echo "test_repo_tools: expected orchestrator dorkpipe $expected_dorkpipe, got $actual_orch" >&2
  exit 1
fi

echo "test_repo_tools OK"
