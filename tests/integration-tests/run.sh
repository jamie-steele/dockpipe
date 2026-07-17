#!/usr/bin/env bash
# Run integration tests. Require Docker. Run from repo root: bash tests/integration-tests/run.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
failed=0
failed_tests=()
log_dir="${TMPDIR:-/tmp}/dockpipe-integration-logs-${RANDOM}-${RANDOM}"
mkdir -p "$log_dir"

# These are source-checkout integration tests. Point the repo-built dockpipe
# binary at this checkout instead of the materialized bundled cache.
export DOCKPIPE_REPO_ROOT="$REPO_ROOT"
export DOCKPIPE_BIN="$REPO_ROOT/src/bin/dockpipe"

# act on Windows runs these Linux tests inside a runner container while Docker
# commands still target the host daemon. Keep temp files under the checkout so
# nested containers can see them, and use root in nested containers so Windows-
# backed mounts stay writable.
if [[ "${ACT:-}" == "true" ]]; then
  export TMPDIR="${REPO_ROOT}/bin/.dockpipe/tmp/act-host"
  export TMP="$TMPDIR"
  export TEMP="$TMPDIR"
  export DOCKPIPE_FORCE_ROOT_CONTAINER=1
  mkdir -p "$TMPDIR"
  chmod 0777 "$TMPDIR" 2>/dev/null || true
fi

if ! command -v docker &>/dev/null; then
  echo "Error: docker not in PATH. Integration tests require Docker." >&2
  exit 1
fi
if ! docker run --rm debian:bookworm-slim true &>/dev/null; then
  echo "Error: docker run failed. Ensure Docker is running and you can run containers." >&2
  exit 1
fi

for f in "$DIR"/test_*.sh; do
  [[ -f "$f" ]] || continue
  name="$(basename "$f")"
  log_file="$log_dir/${name}.log"
  echo "--- $name ---"
  if bash "$f" > >(tee "$log_file") 2> >(tee -a "$log_file" >&2); then
    :
  else
    failed=1
    failed_tests+=("$name")
    echo "$name FAILED"
  fi
done

if [[ $failed -ne 0 ]]; then
  echo "Failed integration tests: ${failed_tests[*]}"
  for name in "${failed_tests[@]}"; do
    log_file="$log_dir/${name}.log"
    echo "--- ${name} failure log tail ---"
    if [[ -f "$log_file" ]]; then
      tail -120 "$log_file"
    else
      echo "missing log file: $log_file"
    fi
  done
fi

if [[ $failed -eq 0 ]]; then
  echo "All integration tests passed."
else
  echo "Some integration tests failed."
  exit 1
fi
