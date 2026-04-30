#!/usr/bin/env bash
# Host-only quick CI emulator for this repo:
# - mirrors the GitHub scan step closely
# - runs repo Go tests after scans pass
# - skips Docker workflow recursion from src/scripts/ci-local.sh
set -euo pipefail

ROOT="$(dockpipe get workdir)"
ROOT="$(cd "$ROOT" && pwd)"
cd "$ROOT"

export DOCKPIPE_WORKDIR="$ROOT"
export DOCKPIPE_REPO_ROOT="${DOCKPIPE_REPO_ROOT:-$ROOT}"
export GOCACHE="${GOCACHE:-$ROOT/bin/.dockpipe/gocache}"

GOBIN="$(go env GOPATH)/bin"
export PATH="$ROOT/src/bin:$GOBIN:$PATH"

have() { command -v "$1" >/dev/null 2>&1; }

need_tool() {
  local tool="$1"
  local hint="$2"
  if ! have "$tool"; then
    printf '[dockpipe] ci-emulate: missing %s. %s\n' "$tool" "$hint" >&2
    exit 1
  fi
}

printf '[dockpipe] ci-emulate: repo root %s\n' "$ROOT" >&2
printf '[dockpipe] ci-emulate: clearing prior scan artifacts under bin/.dockpipe/\n' >&2
rm -rf bin/.dockpipe/ci-raw bin/.dockpipe/ci-analysis
rm -rf .dockpipe/demo bin/.dockpipe/demo
mkdir -p bin/.dockpipe/ci-raw "$GOCACHE"

need_tool go "Install Go and retry."
need_tool govulncheck "Run: make dev-deps  or  bash src/scripts/install-deps.sh"
need_tool gosec "Run: make dev-deps  or  bash src/scripts/install-deps.sh"
need_tool jq "Install jq and retry."

set +e
govulncheck -format json ./... > bin/.dockpipe/ci-raw/govulncheck.json
VC=$?
gosec -conf .gosec.json -fmt json -out=bin/.dockpipe/ci-raw/gosec.json -exclude-dir=.gomodcache ./...
GC=$?
set -e

CI_SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
DOCKPIPE_SCRIPT_DIR="$CI_SCRIPT_DIR" \
  bash "$CI_SCRIPT_DIR/normalize-ci-scans.sh"
printf '[dockpipe] ci-emulate: govulncheck raw findings: '
jq -sr '((([.[] | select(.finding or .Finding)] | length) + ([.[] | (.vulns // .Vulns // [])[]?] | length)) | tostring)' bin/.dockpipe/ci-raw/govulncheck.json 2>/dev/null || true
printf '[dockpipe] ci-emulate: gosec raw issues: '
jq -r '((.Stats.found // 0) | tostring)' bin/.dockpipe/ci-raw/gosec.json 2>/dev/null || true

if [[ $VC -ne 0 ]]; then
  printf '[dockpipe] ci-emulate: govulncheck failed (exit %s)\n' "$VC" >&2
  exit "$VC"
fi
if [[ $GC -ne 0 ]]; then
  printf '[dockpipe] ci-emulate: gosec failed (exit %s)\n' "$GC" >&2
  printf '[dockpipe] ci-emulate: normalized findings: %s\n' "$(jq '.findings | length' bin/.dockpipe/ci-analysis/findings.json 2>/dev/null || echo '?')" >&2
  jq -r '.findings[] | select(.tool=="gosec") | "- [" + (.severity // "?") + "] " + (.rule_id // "?") + " " + (.file // "?") + ":" + ((.line // 0)|tostring) + " — " + (.title // .message // "")' \
    bin/.dockpipe/ci-analysis/findings.json 2>/dev/null || true
  exit "$GC"
fi

printf '[dockpipe] ci-emulate: running go test ./...\n' >&2
go test ./...
printf '[dockpipe] ci-emulate: running dockpipe package test\n' >&2
dockpipe package test --workdir "$ROOT"
printf '[dockpipe] ci-emulate: running dockpipe workflow test\n' >&2
dockpipe workflow test --workdir "$ROOT"

printf '[dockpipe] ci-emulate: complete\n' >&2
