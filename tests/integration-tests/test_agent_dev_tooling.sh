#!/usr/bin/env bash
# Test agent-dev image has Node and Claude CLI (no API call); validates image for AI workflows.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE="$REPO_ROOT/src/bin/dockpipe"
source "$(dirname "${BASH_SOURCE[0]}")/helpers.sh"

require_agent_dev_template "$REPO_ROOT"

# Node is available (agent-dev is Node-based)
out=$("$DOCKPIPE" --no-data --template agent-dev -- node -e "console.log('node ok')")
[[ "$out" == *"node ok"* ]] || { echo "Expected node ok, got: $out"; exit 1; }

# Claude CLI is installed (we only check it exists; no API call)
out=$("$DOCKPIPE" --no-data --template agent-dev -- which claude 2>/dev/null || true)
[[ "$out" == *"claude"* ]] || { echo "Expected claude in PATH, got: $out"; exit 1; }

echo "test_agent_dev_tooling OK"
