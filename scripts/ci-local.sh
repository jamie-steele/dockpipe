#!/usr/bin/env bash
# Mirror the Linux "test" job in .github/workflows/ci.yml (not CodeQL, not Windows).
# From repo root:  make ci   or   bash scripts/ci-local.sh
#
# Optional Codex dogfood (same as CI when vars.DOCKPIPE_CI_CODEX=true):
#   DOCKPIPE_CI_CODEX=true OPENAI_API_KEY=... bash scripts/ci-local.sh
#
# Requires: Go, make, Docker (for workflow + integration tests), dpkg-deb (for .deb build).
# govulncheck / gosec: install with  make dev-deps  or  bash scripts/install-deps.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
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
	echo "ci-local: govulncheck not found. Run:  make dev-deps  or  bash scripts/install-deps.sh" >&2
	exit 1
fi
if ! have gosec; then
	echo "ci-local: gosec not found. Run:  make dev-deps  or  bash scripts/install-deps.sh" >&2
	exit 1
fi

step "govulncheck"
govulncheck ./...

step "gosec"
gosec -conf .gosec.json -fmt text -stdout ./...

step "make (build CLI)"
make

step "go test"
go test ./...

step "templates/core path guard"
bash scripts/check-templates-core-paths.sh

step "dogfood — workflow test (go vet in Docker; mount host module cache)"
./bin/dockpipe --workflow test --runtime docker --workdir "$ROOT" \
	--mount "$(go env GOPATH)/pkg:/go/pkg:rw" \
	--

if [[ "${DOCKPIPE_CI_CODEX:-}" == "true" ]]; then
	step "dogfood — Codex workflows (OPENAI_API_KEY required)"
	if [[ -z "${OPENAI_API_KEY:-}" ]]; then
		echo "ci-local: DOCKPIPE_CI_CODEX=true but OPENAI_API_KEY is empty." >&2
		exit 1
	fi
	export OPENAI_API_KEY
	./bin/dockpipe --workflow dogfood-codex-pav --resolver codex --runtime docker --workdir "$ROOT" --
	./bin/dockpipe --workflow dogfood-codex-security --resolver codex --runtime docker --workdir "$ROOT" --
fi

step "build amd64 .deb"
./packaging/build-deb.sh "$(tr -d ' \t\r\n' < VERSION)" amd64

step "shell unit tests"
bash tests/run_tests.sh

step "integration tests (Docker)"
bash tests/integration-tests/run.sh

echo ""
echo "ci-local: all steps passed (mirrors Linux CI test job; CodeQL runs only on GitHub)."
