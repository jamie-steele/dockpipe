#!/usr/bin/env bash
# Mirror the Linux "test" job in .github/workflows/ci.yml (not CodeQL, not Windows).
# From repo root:  make ci   or   bash src/scripts/ci-local.sh
#
# Optional Codex workflows (same as CI when vars.DOCKPIPE_CI_CODEX=true):
#   DOCKPIPE_CI_CODEX=true OPENAI_API_KEY=... bash src/scripts/ci-local.sh
#
# Requires: Go, make, Docker (for workflow + integration tests), dpkg-deb (for .deb build), jq, shellcheck.
# govulncheck / gosec / staticcheck: install with  make dev-deps  or  bash src/scripts/install-deps.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

export DOCKPIPE_REPO_ROOT="${DOCKPIPE_REPO_ROOT:-$ROOT}"
export DOCKPIPE_WORKFLOW_NAME="${DOCKPIPE_WORKFLOW_NAME:-ci}"

GOBIN="$(go env GOPATH)/bin"
export PATH="$ROOT/src/bin:$GOBIN:$PATH"
eval "$("$ROOT/src/bin/dockpipe" sdk --workdir "$ROOT")"

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
if ! have staticcheck; then
	echo "ci-local: staticcheck not found. Run:  make dev-deps  or  bash src/scripts/install-deps.sh" >&2
	exit 1
fi
if ! have shellcheck; then
	echo "ci-local: shellcheck not found. Install shellcheck with your OS package manager." >&2
	exit 1
fi
if ! have jq; then
	echo "ci-local: jq not found (needed for DorkPipe CI signal bundle). Install jq." >&2
	exit 1
fi

CI_RAW_DIR="$(dockpipe_sdk ci raw)"
CI_ANALYSIS_DIR="$(dockpipe_sdk ci analysis)"

step "govulncheck + gosec + DorkPipe signal bundle ($CI_ANALYSIS_DIR/)"
rm -rf "$CI_RAW_DIR" "$CI_ANALYSIS_DIR"
mkdir -p "$CI_RAW_DIR"
set +e
govulncheck -format json ./... > "$CI_RAW_DIR/govulncheck.json"
VC=$?
gosec -conf .gosec.json -fmt json -out="$CI_RAW_DIR/gosec.json" -exclude-dir=.gomodcache -exclude-dir=bin/.dockpipe ./...
GC=$?
set -e
export DOCKPIPE_WORKDIR="$ROOT"
CI_SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
bash "$CI_SCRIPT_DIR/normalize-ci-scans.sh"
jq -sr '"govulncheck raw findings: " + ((([.[] | select(.finding or .Finding)] | length) + ([.[] | (.vulns // .Vulns // [])[]?] | length)) | tostring)' "$CI_RAW_DIR/govulncheck.json" 2>/dev/null || true
jq -r '"gosec raw issues: " + ((.Stats.found // 0) | tostring)' "$CI_RAW_DIR/gosec.json" 2>/dev/null || true
if [[ $VC -ne 0 ]]; then exit "$VC"; fi
if [[ $GC -ne 0 ]]; then exit "$GC"; fi

step "make (build CLI)"
make

step "go test"
go test ./...

step "staticcheck"
staticcheck ./...

step "shellcheck (syntax errors)"
mapfile -t shell_scripts < <(git ls-files '*.sh' ':!:src/app/tooling/vscode-extensions/**/node_modules/**' ':!:bin/**')
existing_shell_scripts=()
for script in "${shell_scripts[@]}"; do
	[[ -f "$script" ]] && existing_shell_scripts+=("$script")
done
if ((${#existing_shell_scripts[@]})); then
	shellcheck -S error "${existing_shell_scripts[@]}"
fi

step "dockpipe package test"
./src/bin/dockpipe package test --workdir "$ROOT"

step "dockpipe workflow test"
./src/bin/dockpipe workflow test --workdir "$ROOT"

step "templates/core path guard"
bash src/scripts/check-templates-core-paths.sh

step "workflow test (go test + vet + govulncheck + gosec in Docker; mount module cache only)"
./src/bin/dockpipe --workflow test --runtime docker --workdir "$ROOT" \
	--mount "$(go env GOPATH)/pkg:/go/pkg:rw" \
	--

if [[ "${DOCKPIPE_CI_CODEX:-}" == "true" ]]; then
	step "Codex workflows — codex-pav, codex-security (OPENAI_API_KEY required)"
	if [[ -z "${OPENAI_API_KEY:-}" ]]; then
		echo "ci-local: DOCKPIPE_CI_CODEX=true but OPENAI_API_KEY is empty." >&2
		exit 1
	fi
	export OPENAI_API_KEY
	./src/bin/dockpipe --workflow codex-pav --resolver codex --runtime docker --workdir "$ROOT" --
	./src/bin/dockpipe --workflow codex-security --resolver codex --runtime docker --workdir "$ROOT" --
fi

step "build amd64 .deb"
./release/packaging/build-deb.sh "$(tr -d ' \t\r\n' < VERSION)" amd64

step "shell unit tests"
bash tests/run_tests.sh

step "integration tests (Docker)"
bash tests/integration-tests/run.sh

echo ""
echo "ci-local: all steps passed (mirrors Linux CI test job; CodeQL runs only on GitHub)."
