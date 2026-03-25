#!/usr/bin/env bash
# Guardrails for repo layout after moves (src/templates, workflows/, Pipeon asset paths).
# Run from repo root: bash tests/unit-tests/test_repo_layout.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

fail() {
	echo "test_repo_layout: FAIL — $1" >&2
	exit 1
}

test -d "$ROOT/src/templates/core" || fail "missing src/templates/core"
test -f "$ROOT/src/templates/run/config.yml" || fail "missing bundled workflow src/templates/run/config.yml"
test -d "$ROOT/workflows" || fail "missing workflows/"
test -f "$ROOT/embed.go" || fail "missing repo-root embed.go"

# Pipeon harness (src/bin/pipeon → src/pipeon/scripts/pipeon.sh) must resolve to the bundle
test -f "$ROOT/src/pipeon/scripts/pipeon.sh" || fail "missing src/pipeon/scripts/pipeon.sh (symlink to src/templates/core/bundles/pipeon/…)"

# Pipeon desktop / icon pipeline must not point at removed top-level templates/
test -f "$ROOT/src/templates/core/resolvers/code-server/assets/images/code-server/favicon.svg" \
	|| fail "missing code-server favicon.svg (Pipeon shortcut + make pipeon-icons)"

if ! grep -q 'ICON_SVG=.*src/templates/core/resolvers/code-server' "$ROOT/src/pipeon/scripts/install-pipeon-desktop-shortcut.sh"; then
	fail "install-pipeon-desktop-shortcut.sh ICON_SVG must use src/templates/core/resolvers/code-server/…"
fi
if grep -q 'os.path.join(REPO_ROOT, "templates"' "$ROOT/src/pipeon/scripts/generate-pipeon-icons.py" 2>/dev/null; then
	fail "generate-pipeon-icons.py must not join REPO_ROOT to top-level templates/ — use src/templates/…"
fi
if ! grep -q '"src"' "$ROOT/src/pipeon/scripts/generate-pipeon-icons.py" || ! grep -q '"resolvers"' "$ROOT/src/pipeon/scripts/generate-pipeon-icons.py"; then
	fail "generate-pipeon-icons.py must build path under src/templates/…/resolvers/code-server/…"
fi

echo "test_repo_layout OK"
