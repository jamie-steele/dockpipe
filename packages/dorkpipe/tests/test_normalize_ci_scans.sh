#!/usr/bin/env bash
# Smoke test for dorkpipe resolver normalize-ci-scans.sh (requires jq).
# Run from repo root: bash packages/dorkpipe/tests/test_normalize_ci_scans.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR" && git rev-parse --show-toplevel)"
cd "$ROOT"

if ! command -v jq >/dev/null 2>&1; then
	echo "test_normalize_ci_scans: skip (jq not installed)"
	exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/bin/.dockpipe/ci-raw"
echo '{"Issues":[],"Stats":{"found":0},"GosecVersion":"fixture"}' >"$tmp/bin/.dockpipe/ci-raw/gosec.json"
echo '{"config":{"scanner_version":"fixture"},"vulns":[]}' >"$tmp/bin/.dockpipe/ci-raw/govulncheck.json"

export DOCKPIPE_WORKDIR="$tmp"
bash "$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts/normalize-ci-scans.sh"

if ! jq -e '.schema_version == "1.0" and (.findings | type == "array")' "$tmp/bin/.dockpipe/ci-analysis/findings.json" >/dev/null; then
	echo "test_normalize_ci_scans: findings.json shape unexpected" >&2
	exit 1
fi

echo "test_normalize_ci_scans OK"
