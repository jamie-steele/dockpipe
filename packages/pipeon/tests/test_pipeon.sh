#!/usr/bin/env bash
# Pipeon: feature gate + bundle smoke (no Ollama required).
# Run from repo root: bash packages/pipeon/tests/test_pipeon.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR" && git rev-parse --show-toplevel)"
cd "$ROOT"
PIPEON="$ROOT/packages/pipeon/bin/pipeon"
test -x "$PIPEON" || {
	echo "test_pipeon: missing $PIPEON" >&2
	exit 1
}

# Without DOCKPIPE_PIPEON, bundle must fail
set +e
DOCKPIPE_WORKDIR="$ROOT" "$PIPEON" bundle >/dev/null 2>&1
rc=$?
set -e
if [[ "$rc" -eq 0 ]]; then
	echo "test_pipeon: expected bundle to fail without DOCKPIPE_PIPEON" >&2
	exit 1
fi

export DOCKPIPE_PIPEON=1
export DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1
export DOCKPIPE_WORKDIR="$ROOT"

if ! "$PIPEON" status >/dev/null; then
	echo "test_pipeon: status failed with flags set" >&2
	exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export DOCKPIPE_WORKDIR="$tmp"

if ! "$PIPEON" bundle >/dev/null; then
	echo "test_pipeon: bundle failed in empty repo" >&2
	exit 1
fi

if [[ ! -f "$tmp/.dockpipe/pipeon-context.md" ]]; then
	echo "test_pipeon: missing pipeon-context.md" >&2
	exit 1
fi

if ! grep -q 'Pipeon context bundle' "$tmp/.dockpipe/pipeon-context.md"; then
	echo "test_pipeon: unexpected bundle content" >&2
	exit 1
fi

echo "test_pipeon OK"
