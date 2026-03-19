#!/usr/bin/env bash
# Test default (base-dev) template: no --template, just run a command.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE="$REPO_ROOT/bin/dockpipe"

# (stdout may include image digest or container name from quick-exit dump; just assert expected output appears)
out=$("$DOCKPIPE" --no-data -- echo "hello from default")
[[ "$out" == *"hello from default"* ]] || { echo "Expected 'hello from default' in output, got: $out"; exit 1; }

echo "test_default_template OK"
