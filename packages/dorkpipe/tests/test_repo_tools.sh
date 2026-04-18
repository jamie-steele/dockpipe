#!/usr/bin/env bash
# Shared DockPipe helper should prefer the repo-local dockpipe build, and the
# DorkPipe package helper should resolve the package-local dorkpipe tool.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# shellcheck source=/dev/null
source "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh"
# shellcheck source=/dev/null
source "$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts/lib/dorkpipe-cli.sh"

expected_dockpipe="$ROOT/src/bin/dockpipe"
actual_dockpipe="$(dockpipe_sdk require dockpipe-bin)"
if [[ "$actual_dockpipe" != "$expected_dockpipe" ]]; then
  echo "test_repo_tools: expected dockpipe $expected_dockpipe, got $actual_dockpipe" >&2
  exit 1
fi

expected_dorkpipe="$ROOT/packages/dorkpipe/bin/dorkpipe"
actual_dorkpipe="$(dorkpipe_script_resolve_bin "$ROOT")"
if [[ "$actual_dorkpipe" != "$expected_dorkpipe" ]]; then
  echo "test_repo_tools: expected dorkpipe $expected_dorkpipe, got $actual_dorkpipe" >&2
  exit 1
fi

actual_orch="$(dorkpipe_script_resolve_bin "$ROOT")"
if [[ "$actual_orch" != "$expected_dorkpipe" ]]; then
  echo "test_repo_tools: expected orchestrator dorkpipe $expected_dorkpipe, got $actual_orch" >&2
  exit 1
fi

echo "test_repo_tools OK"
