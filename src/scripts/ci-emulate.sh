#!/usr/bin/env bash
# Run the real GitHub Actions CI job locally through nektos/act.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

JOB="${DOCKPIPE_CI_EMULATE_JOB:-test}"
EVENT="${DOCKPIPE_CI_EMULATE_EVENT:-pull_request}"
WORKFLOW="${DOCKPIPE_CI_EMULATE_WORKFLOW:-.github/workflows/ci.yml}"
PLATFORM="${DOCKPIPE_CI_EMULATE_PLATFORM:-ghcr.io/catthehacker/ubuntu:act-22.04}"

have() { command -v "$1" >/dev/null 2>&1; }

if ! have docker; then
	echo "ci-emulate: docker is required" >&2
	exit 1
fi
if ! have act; then
	cat >&2 <<'MSG'
ci-emulate: act is not installed.

Install nektos/act, then rerun:
  ./src/bin/dockpipe --workflow ci-emulate --workdir . --

Examples:
  Windows: winget install nektos.act
  macOS:   brew install act
  Linux:   see https://github.com/nektos/act#installation
MSG
	exit 1
fi

echo "ci-emulate: event=$EVENT job=$JOB workflow=$WORKFLOW platform=$PLATFORM"
echo "ci-emulate: running the real GitHub Actions job locally; this may pull runner/action images"
exec act "$EVENT" \
	-W "$WORKFLOW" \
	-j "$JOB" \
	-P "ubuntu-latest=$PLATFORM" \
	--artifact-server-path "$ROOT/bin/.dockpipe/act-artifacts"
