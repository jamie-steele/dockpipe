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

# Pipeon harness (src/bin/pipeon → src/apps/pipeon/scripts/pipeon.sh) must resolve to the bundle
test -f "$ROOT/src/apps/pipeon/scripts/pipeon.sh" || fail "missing src/apps/pipeon/scripts/pipeon.sh (symlink to .staging/bundles/pipeon/…)"

# Pipeon desktop / icon pipeline must not point at removed top-level templates/
test -f "$ROOT/.staging/resolvers/code-server/assets/images/code-server/favicon.svg" \
	|| fail "missing code-server favicon.svg (Pipeon shortcut + make pipeon-icons)"

if ! grep -q 'ICON_SVG=.*\.staging/resolvers/code-server' "$ROOT/src/apps/pipeon/scripts/install-pipeon-desktop-shortcut.sh"; then
	fail "install-pipeon-desktop-shortcut.sh ICON_SVG must use .staging/resolvers/code-server/…"
fi
if grep -q 'os.path.join(REPO_ROOT, "templates"' "$ROOT/src/apps/pipeon/scripts/generate-pipeon-icons.py" 2>/dev/null; then
	fail "generate-pipeon-icons.py must not join REPO_ROOT to top-level templates/"
fi
if ! grep -q '".staging"' "$ROOT/src/apps/pipeon/scripts/generate-pipeon-icons.py" || ! grep -q '"resolvers"' "$ROOT/src/apps/pipeon/scripts/generate-pipeon-icons.py"; then
	fail "generate-pipeon-icons.py must build path under .staging/resolvers/code-server/…"
fi

echo "test_repo_layout OK"
