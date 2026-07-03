#!/usr/bin/env bash
# Smoke test for the normalize-ci-scans wrapper around `dorkpipe ci normalize-scans`.
# Run from repo root: bash packages/dorkpipe/tests/test_normalize_ci_scans.sh
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
export PATH="$ROOT/src/bin${PATH:+:$PATH}"
# shellcheck source=packages/dorkpipe/tests/lib/test-tools.sh
source "$ROOT/packages/dorkpipe/tests/lib/test-tools.sh"
dorkpipe_test_require_go "test_normalize_ci_scans"
dorkpipe_test_init_go_cache "$ROOT"

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

if ! dorkpipe_test_assert "$ROOT" normalize-findings "$ci_root/ci-analysis/findings.json"; then
	echo "test_normalize_ci_scans: findings.json shape unexpected" >&2
	exit 1
fi
if ! dorkpipe_test_assert "$ROOT" normalize-gosec "$ci_root/ci-analysis/raw/gosec.json"; then
	echo "test_normalize_ci_scans: raw/gosec.json was not refreshed" >&2
	exit 1
fi

echo "test_normalize_ci_scans OK"
