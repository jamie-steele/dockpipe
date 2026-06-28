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

ci_root="$tmp/ci-artifacts"
mkdir -p "$ci_root/ci-raw"
echo '{"Issues":[],"Stats":{"found":0},"GosecVersion":"fixture"}' >"$ci_root/ci-raw/gosec.json"
echo '{"config":{"scanner_version":"fixture"},"vulns":[]}' >"$ci_root/ci-raw/govulncheck.json"
mkdir -p "$ci_root/ci-analysis/raw"
echo 'stale-findings' >"$ci_root/ci-analysis/findings.json"
echo 'stale-summary' >"$ci_root/ci-analysis/SUMMARY.md"
echo 'stale-raw' >"$ci_root/ci-analysis/raw/gosec.json"

export DOCKPIPE_WORKDIR="$tmp"
export DOCKPIPE_CI_RAW_DIR="$ci_root/ci-raw"
export DOCKPIPE_CI_ANALYSIS_DIR="$ci_root/ci-analysis"
export DOCKPIPE_SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
bash "$DOCKPIPE_SCRIPT_DIR/normalize-ci-scans.sh"

if ! jq -e '.schema_version == "1.0" and (.findings | type == "array")' "$ci_root/ci-analysis/findings.json" >/dev/null; then
	echo "test_normalize_ci_scans: findings.json shape unexpected" >&2
	exit 1
fi
if ! jq -e '.Issues | type == "array"' "$ci_root/ci-analysis/raw/gosec.json" >/dev/null; then
	echo "test_normalize_ci_scans: raw/gosec.json was not refreshed" >&2
	exit 1
fi

echo "test_normalize_ci_scans OK"
