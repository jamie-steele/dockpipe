#!/usr/bin/env bash
# Quick checks: Go tests + template path guard + bash unit tests (no Docker, no integration).
# From repo root:  make test-quick
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
export DOCKPIPE_REPO_ROOT="${DOCKPIPE_REPO_ROOT:-$ROOT}"

UNIT="$(cd "${ROOT}/tests/unit-tests" && pwd)"

echo "=== go test (root + maintainer modules) ==="
go test ./...
go test ./packages/dorkpipe/lib/...
go test ./packages/dockpipe-mcp/...

echo "=== check-templates-core-paths ==="
bash src/scripts/check-templates-core-paths.sh

for f in test_cli.sh test_repo_root.sh test_repo_layout.sh test_clone_worktree_include.sh; do
	echo "=== $f ==="
	bash "${UNIT}/${f}"
done

echo ""
echo "test-quick: ok"
