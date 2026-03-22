#!/usr/bin/env bash
# Smoke test for scripts/dorkpipe/normalize-ci-scans.sh (requires jq).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if ! command -v jq >/dev/null 2>&1; then
	echo "test_normalize_ci_scans: skip (jq not installed)"
	exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/.dockpipe/ci-raw"
echo '{"Issues":[],"Stats":{"found":0},"GosecVersion":"fixture"}' >"$tmp/.dockpipe/ci-raw/gosec.json"
echo '{"config":{"scanner_version":"fixture"},"vulns":[]}' >"$tmp/.dockpipe/ci-raw/govulncheck.json"

export DOCKPIPE_WORKDIR="$tmp"
bash "$ROOT/scripts/dorkpipe/normalize-ci-scans.sh"

if ! jq -e '.schema_version == "1.0" and (.findings | type == "array")' "$tmp/.dockpipe/ci-analysis/findings.json" >/dev/null; then
	echo "test_normalize_ci_scans: findings.json shape unexpected" >&2
	exit 1
fi

echo "test_normalize_ci_scans OK"
