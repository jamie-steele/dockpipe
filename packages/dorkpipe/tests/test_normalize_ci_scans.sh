#!/usr/bin/env bash
# Smoke test for the normalize-ci-scans wrapper around `dorkpipe ci normalize-scans` (jq used for assertions).
# Run from repo root: bash packages/dorkpipe/tests/test_normalize_ci_scans.sh
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
export PATH="$ROOT/src/bin${PATH:+:$PATH}"

if ! command -v jq >/dev/null 2>&1; then
	echo "test_normalize_ci_scans: skip (jq not installed)"
	exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/bin/.dockpipe/ci-raw"
echo '{"Issues":[],"Stats":{"found":0},"GosecVersion":"fixture"}' >"$tmp/bin/.dockpipe/ci-raw/gosec.json"
echo '{"config":{"scanner_version":"fixture"},"vulns":[]}' >"$tmp/bin/.dockpipe/ci-raw/govulncheck.json"
mkdir -p "$tmp/bin/.dockpipe/ci-analysis/raw"
echo 'stale-findings' >"$tmp/bin/.dockpipe/ci-analysis/findings.json"
echo 'stale-summary' >"$tmp/bin/.dockpipe/ci-analysis/SUMMARY.md"
echo 'stale-raw' >"$tmp/bin/.dockpipe/ci-analysis/raw/gosec.json"

export DOCKPIPE_WORKDIR="$tmp"
export DOCKPIPE_SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
bash "$DOCKPIPE_SCRIPT_DIR/normalize-ci-scans.sh"

if ! jq -e '.schema_version == "1.0" and (.findings | type == "array")' "$tmp/bin/.dockpipe/ci-analysis/findings.json" >/dev/null; then
	echo "test_normalize_ci_scans: findings.json shape unexpected" >&2
	exit 1
fi
if ! jq -e '.Issues | type == "array"' "$tmp/bin/.dockpipe/ci-analysis/raw/gosec.json" >/dev/null; then
	echo "test_normalize_ci_scans: raw/gosec.json was not refreshed" >&2
	exit 1
fi

echo "test_normalize_ci_scans OK"
