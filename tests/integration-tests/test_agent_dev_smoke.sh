#!/usr/bin/env bash
# Smoke test: agent-dev template runs, data volume is mounted, --no-data disables it.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE="$REPO_ROOT/bin/dockpipe"

# 1. Default: data volume mounted, DOCKPIPE_DATA set
out=$("$DOCKPIPE" --template agent-dev -- sh -c 'echo "workdir=$PWD"; echo "data=${DOCKPIPE_DATA:-not set}"; test -d /dockpipe-data && echo "volume_ok"')
echo "$out" | grep -q "workdir=/work" || { echo "Expected workdir=/work"; exit 1; }
echo "$out" | grep -q "data=/dockpipe-data" || { echo "Expected data=/dockpipe-data"; exit 1; }
echo "$out" | grep -q "volume_ok" || { echo "Expected /dockpipe-data to exist"; exit 1; }

# 2. --no-data: no volume
out=$("$DOCKPIPE" --no-data --template agent-dev -- sh -c 'echo "data=${DOCKPIPE_DATA:-not set}"; test -d /dockpipe-data && echo "volume_ok" || echo "no_volume"')
echo "$out" | grep -q "no_volume" || { echo "Expected no /dockpipe-data when --no-data"; exit 1; }

echo "test_agent_dev_smoke OK"
