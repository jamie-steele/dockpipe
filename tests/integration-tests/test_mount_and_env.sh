#!/usr/bin/env bash
# Test --mount (extra volume) and --env (pass env into container).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE="$REPO_ROOT/bin/dockpipe"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# 1. --mount: bind a file into the container
# (stdout may include image digest or container name; assert expected content appears)
echo "mount_content_here" > "$tmp/mounted.txt"
out=$("$DOCKPIPE" --no-data --template agent-dev --mount "$tmp/mounted.txt:/tmp/mounted.txt" -- cat /tmp/mounted.txt)
[[ "$out" == *"mount_content_here"* ]] || { echo "Expected mounted content in output, got: $out"; exit 1; }

# 2. --env: pass env var
out=$("$DOCKPIPE" --no-data --template agent-dev --env "INTTEST_FOO=bar" -- sh -c 'echo "$INTTEST_FOO"')
[[ "$out" == *"bar"* ]] || { echo "Expected env FOO=bar in output, got: $out"; exit 1; }

echo "test_mount_and_env OK"
