#!/usr/bin/env bash
# Test -d / --detach: container runs in background, stdout shows container ID.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCKPIPE="$REPO_ROOT/bin/dockpipe"

# Run detached: sleep 2 so the container stays up briefly; we capture the ID
# (stdout may include image digest from build; container ID is the last line)
out=$("$DOCKPIPE" -d --no-data --template agent-dev -- sleep 2)
cid=$(echo "$out" | tail -1 | tr -d '\n')
# Container ID is 12 or 64 hex chars (docker run -d prints short id)
[[ "$cid" =~ ^[a-f0-9]{12,64}$ ]] || { echo "Expected container ID, got: $cid"; exit 1; }
# Container will exit when sleep ends; optional: wait and rm if still present
docker rm -f "$cid" 2>/dev/null || true

echo "test_detach OK"
