#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
DOCKPIPE_BIN="${ROOT}/src/bin/dockpipe"

if [[ ! -x "${DOCKPIPE_BIN}" ]]; then
  echo "packages/agent/tests/run.sh: missing ${DOCKPIPE_BIN}; run make build first" >&2
  exit 1
fi

for workflow_path in \
  "workflows/agent.cloud-lanes.doctor/config.yml" \
  "workflows/docs.orchestrate/config.yml" \
  "workflows/docs.optimize-orchestrate/config.yml"; do
  echo "--- validate ${workflow_path} ---"
  "${DOCKPIPE_BIN}" workflow validate "${workflow_path}"
done

echo "--- check promoted files exclude junk ---"
if find "${ROOT}/packages/agent" -type f \( \
  -path '*/bin/*' -o -path '*/obj/*' -o -path '*/dist/*' -o -path '*/build/*' -o \
  -path '*/node_modules/*' -o -path '*/.venv/*' -o -path '*/__pycache__/*' -o \
  -name '*.exe' -o -name '*.dll' -o -name '*.so' -o -name '*.dylib' -o \
  -name '*.zip' -o -name '*.tar' -o -name '*.7z' -o -name '*.log' -o -name '.env' \
  \) | grep -q .; then
  echo "packages/agent/tests/run.sh: unexpected generated or binary artifact under packages/agent" >&2
  exit 1
fi

echo "packages/agent/tests/run.sh OK"
