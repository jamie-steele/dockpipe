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
test -f "$ROOT/src/apps/pipeon/scripts/pipeon.sh" || fail "missing src/apps/pipeon/scripts/pipeon.sh (symlink to .staging/packages/dockpipe/ide/pipeon/…)"

# Pipeon desktop / icon pipeline must not point at removed top-level templates/
test -f "$ROOT/src/contrib/pipeon-vscode-extension/images/favicon.svg" \
	|| fail "missing Pipeon favicon.svg (run: make pipeon-icons)"

if ! grep -q 'ICON_SVG=.*src/contrib/pipeon-vscode-extension/images/favicon.svg' "$ROOT/src/apps/pipeon/scripts/install-pipeon-desktop-shortcut.sh"; then
	fail "install-pipeon-desktop-shortcut.sh ICON_SVG must use src/contrib/pipeon-vscode-extension/images/favicon.svg"
fi
if grep -q 'os.path.join(REPO_ROOT, "templates"' "$ROOT/src/apps/pipeon/scripts/generate-pipeon-icons.py" 2>/dev/null; then
	fail "generate-pipeon-icons.py must not join REPO_ROOT to top-level templates/"
fi
if ! grep -q 'pipeon-vscode-extension' "$ROOT/src/apps/pipeon/scripts/generate-pipeon-icons.py" || ! grep -q '"images"' "$ROOT/src/apps/pipeon/scripts/generate-pipeon-icons.py"; then
	fail "generate-pipeon-icons.py must write under src/contrib/pipeon-vscode-extension/images/"
fi

echo "test_repo_layout OK"
