#!/usr/bin/env bash
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
HARNESS="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/cas01/app_server.go"
FIXTURES="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/cas01/fixtures/fixtures.json"
go run "$HARNESS" --mode fixtures --fixtures "$FIXTURES"
go run "$HARNESS" --mode diagnostics
if go run "$HARNESS" --mode policy --sandbox danger-full-access >/dev/null 2>&1; then echo "CAS-01 accepted full access" >&2; exit 1; fi
if go run "$HARNESS" --mode policy --method thread/shellCommand >/dev/null 2>&1; then echo "CAS-01 accepted shell shortcut" >&2; exit 1; fi
if go run "$HARNESS" --mode policy --reviewer auto_review >/dev/null 2>&1; then echo "CAS-01 accepted auto-review" >&2; exit 1; fi
if go run "$HARNESS" --mode policy --model gpt-5.6-sol >/dev/null 2>&1; then echo "CAS-01 accepted GPT-5.6 Sol" >&2; exit 1; fi
if go run "$HARNESS" --mode policy --reasoning-effort xhigh >/dev/null 2>&1; then echo "CAS-01 accepted an unselected reasoning effort" >&2; exit 1; fi
echo "dorkpipe/tests/test_cas01_app_server.sh OK"
