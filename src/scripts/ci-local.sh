#!/usr/bin/env bash
# Mirror the Linux "test" job in .github/workflows/ci.yml (not CodeQL, not Windows).
# From repo root:  make ci   or   bash src/scripts/ci-local.sh
#
# Optional Codex dogfood (same as CI when vars.DOCKPIPE_CI_CODEX=true):
#   DOCKPIPE_CI_CODEX=true OPENAI_API_KEY=... bash src/scripts/ci-local.sh
#
# Requires: Go, make, Docker (for workflow + integration tests), dpkg-deb (for .deb build), jq.
# govulncheck / gosec: install with  make dev-deps  or  bash src/scripts/install-deps.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

export DOCKPIPE_REPO_ROOT="${DOCKPIPE_REPO_ROOT:-$ROOT}"

GOBIN="$(go env GOPATH)/bin"
export PATH="$GOBIN:$PATH"

have() { command -v "$1" >/dev/null 2>&1; }

step() {
	echo ""
	echo "=== $* ==="
}

if ! have govulncheck; then
	echo "ci-local: govulncheck not found. Run:  make dev-deps  or  bash src/scripts/install-deps.sh" >&2
	exit 1
fi
if ! have gosec; then
	echo "ci-local: gosec not found. Run:  make dev-deps  or  bash src/scripts/install-deps.sh" >&2
	exit 1
fi
if ! have jq; then
	echo "ci-local: jq not found (needed for DorkPipe CI signal bundle). Install jq." >&2
	exit 1
fi

step "govulncheck + gosec + DorkPipe signal bundle (.dockpipe/ci-analysis/)"
mkdir -p .dockpipe/ci-raw
set +e
govulncheck -format json ./... > .dockpipe/ci-raw/govulncheck.json
VC=$?
gosec -conf .gosec.json -fmt json -out=.dockpipe/ci-raw/gosec.json ./...
GC=$?
set -e
export DOCKPIPE_WORKDIR="$ROOT"
bash src/scripts/dorkpipe/normalize-ci-scans.sh
jq -r '"govulncheck raw vulns: " + ((.vulns // .Vulns // [] | length) | tostring)' .dockpipe/ci-raw/govulncheck.json 2>/dev/null || true
jq -r '"gosec raw issues: " + ((.Stats.found // 0) | tostring)' .dockpipe/ci-raw/gosec.json 2>/dev/null || true
if [[ $VC -ne 0 ]]; then exit "$VC"; fi
if [[ $GC -ne 0 ]]; then exit "$GC"; fi

step "make (build CLI)"
make

step "go test"
go test ./...

step "templates/core path guard"
bash src/scripts/check-templates-core-paths.sh

step "dogfood — workflow test (go test + vet + govulncheck + gosec in Docker; mount module cache only)"
./src/bin/dockpipe --workflow test --runtime docker --workdir "$ROOT" \
	--mount "$(go env GOPATH)/pkg:/go/pkg:rw" \
	--

if [[ "${DOCKPIPE_CI_CODEX:-}" == "true" ]]; then
	step "dogfood — Codex workflows (OPENAI_API_KEY required)"
	if [[ -z "${OPENAI_API_KEY:-}" ]]; then
		echo "ci-local: DOCKPIPE_CI_CODEX=true but OPENAI_API_KEY is empty." >&2
		exit 1
	fi
	export OPENAI_API_KEY
	./src/bin/dockpipe --workflow dogfood-codex-pav --resolver codex --runtime docker --workdir "$ROOT" --
	./src/bin/dockpipe --workflow dogfood-codex-security --resolver codex --runtime docker --workdir "$ROOT" --
fi

step "build amd64 .deb"
./release/packaging/build-deb.sh "$(tr -d ' \t\r\n' < VERSION)" amd64

step "shell unit tests"
bash tests/run_tests.sh

step "integration tests (Docker)"
bash tests/integration-tests/run.sh

echo ""
echo "ci-local: all steps passed (mirrors Linux CI test job; CodeQL runs only on GitHub)."
