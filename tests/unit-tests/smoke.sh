#!/usr/bin/env bash
# Smoke test: run a trivial command in the default image (builds if needed).
# Requires Docker. Skip if docker not available.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLI="${REPO_ROOT}/src/bin/dockpipe"

if ! command -v docker &>/dev/null; then
  echo "smoke: Docker not found, skip"
  exit 0
fi

if ! docker info &>/dev/null; then
  echo "smoke: Docker daemon not reachable, skip"
  exit 0
fi

# From repo root so default image can be built (needs context for COPY assets/entrypoint.sh)
cd "$REPO_ROOT"
out=$("$CLI" -- ls -la /work 2>&1)
if echo "$out" | grep -q "Dockerfile\|entrypoint\|README"; then
  echo "smoke OK (ls /work showed repo contents)"
else
  echo "smoke FAIL: unexpected output"
  echo "$out"
  exit 1
fi
