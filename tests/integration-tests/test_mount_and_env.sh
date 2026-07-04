#!/usr/bin/env bash
# Test --mount (extra volume) and --env (pass env into container).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE="$REPO_ROOT/src/bin/dockpipe"
source "$(dirname "${BASH_SOURCE[0]}")/helpers.sh"

require_agent_dev_template "$REPO_ROOT"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mount_host="$tmp"
case "${OS:-}" in
  Windows_NT)
    if command -v cygpath >/dev/null 2>&1; then
      mount_host="$(cygpath -aw "$tmp")"
    fi
    ;;
esac

# 1. --mount: bind a host directory into the container.
# Windows Docker Desktop is more reliable with directory binds than single-file binds, while still
# covering the same DockPipe mount path.
echo "mount_content_here" > "$tmp/mounted.txt"
out=$(MSYS2_ARG_CONV_EXCL='*' "$DOCKPIPE" --no-data --template agent-dev --mount "$mount_host:/tmp/inttest" -- sh -c 'cat /tmp/inttest/mounted.txt')
[[ "$out" == *"mount_content_here"* ]] || { echo "Expected mounted content in output, got: $out"; exit 1; }

# 2. --env: pass env var
out=$("$DOCKPIPE" --no-data --template agent-dev --env "INTTEST_FOO=bar" -- sh -c 'echo "$INTTEST_FOO"')
[[ "$out" == *"bar"* ]] || { echo "Expected env FOO=bar in output, got: $out"; exit 1; }

echo "test_mount_and_env OK"
