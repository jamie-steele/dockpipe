#!/usr/bin/env bash
# Guardrails for repo layout (src/core, workflows/, Pipeon asset paths).
# Run from repo root: bash tests/unit-tests/test_repo_layout.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

fail() {
	echo "test_repo_layout: FAIL — $1" >&2
	exit 1
}

test -d "$ROOT/src/core/runtimes" || fail "missing src/core/runtimes"
test -f "$ROOT/src/core/workflows/run/config.yml" || fail "missing bundled workflow src/core/workflows/run/config.yml"
test -d "$ROOT/workflows" || fail "missing workflows/"
test -f "$ROOT/embed.go" || fail "missing repo-root embed.go"

test ! -L "$ROOT/src/scripts/pipeon" || fail "src/scripts/pipeon must not be a symlink (use paths.go + compile.workflows — see paths_test.go)"
test ! -e "$ROOT/src/scripts/review" || fail "src/scripts/review removed — use workflows/review-pipeline/ and logical scripts/review-pipeline/… (paths_test.go)"
test ! -d "$ROOT/src/scripts/dockpipe" || fail "src/scripts/dockpipe removed — maintainer scripts live under packages/dorkpipe/resolvers/dorkpipe/assets/scripts/ (logical scripts/dockpipe/…)"
test ! -e "$ROOT/src/bin/dorkpipe" || fail "src/bin/dorkpipe must not exist — use packages/dorkpipe/bin/dorkpipe"
test ! -e "$ROOT/src/bin/mcpd" || fail "src/bin/mcpd must not exist — use packages/dockpipe-mcp/bin/mcpd"
test ! -e "$ROOT/src/bin/pipeon" || fail "src/bin/pipeon must not exist — use packages/pipeon/bin/pipeon"

# Pipeon (first-party — packages/pipeon/)
test -f "$ROOT/packages/pipeon/bin/pipeon" || fail "missing packages/pipeon/bin/pipeon"
test -f "$ROOT/packages/pipeon/resolvers/pipeon/assets/scripts/pipeon.sh" || fail "missing pipeon resolver pipeon.sh"
test -f "$ROOT/packages/pipeon/resolvers/pipeon/vscode-extension/images/favicon.svg" \
	|| fail "missing Pipeon favicon.svg (run: make pipeon-icons)"
if grep -q 'os.path.join(REPO_ROOT, "templates"' "$ROOT/packages/pipeon/resolvers/pipeon/assets/scripts/generate-pipeon-icons.py" 2>/dev/null; then
	fail "generate-pipeon-icons.py must not join REPO_ROOT to top-level templates/"
fi
if ! grep -q 'vscode-extension' "$ROOT/packages/pipeon/resolvers/pipeon/assets/scripts/generate-pipeon-icons.py" || ! grep -q '"images"' "$ROOT/packages/pipeon/resolvers/pipeon/assets/scripts/generate-pipeon-icons.py"; then
	fail "generate-pipeon-icons.py must write under pipeon vscode-extension/images/"
fi

echo "test_repo_layout OK"
