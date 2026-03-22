#!/usr/bin/env bash
# Pipeon: feature gate + bundle smoke (no Ollama required).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

# Without DOCKPIPE_PIPEON, bundle must fail
set +e
DOCKPIPE_WORKDIR="$ROOT" "$ROOT/bin/pipeon" bundle >/dev/null 2>&1
rc=$?
set -e
if [[ "$rc" -eq 0 ]]; then
	echo "test_pipeon: expected bundle to fail without DOCKPIPE_PIPEON" >&2
	exit 1
fi

export DOCKPIPE_PIPEON=1
export DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1
export DOCKPIPE_WORKDIR="$ROOT"

if ! "$ROOT/bin/pipeon" status >/dev/null; then
	echo "test_pipeon: status failed with flags set" >&2
	exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export DOCKPIPE_WORKDIR="$tmp"

if ! "$ROOT/bin/pipeon" bundle >/dev/null; then
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
