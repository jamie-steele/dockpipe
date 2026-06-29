#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
[[ -f "${DORKPIPE_ORCH_PLAN_JSON}" ]] || { echo "missing plan artifact: ${DORKPIPE_ORCH_PLAN_JSON}" >&2; exit 1; }

if [[ "${DORKPIPE_ORCH_SKIP_APPLY:-0}" =~ ^(1|true|yes|on)$ ]]; then
  mkdir -p "${DORKPIPE_ORCH_APPLY_DIR}"
  cat > "${DORKPIPE_ORCH_APPLY_DIR}/result.json" <<EOF
{
  "status": "skipped",
  "reason": "DORKPIPE_ORCH_SKIP_APPLY is enabled",
  "applied": []
}
EOF
  printf '[dorkpipe] apply skipped at %s\n' "${DORKPIPE_ORCH_APPLY_DIR}/result.json" >&2
  exit 0
fi

"$(dorkpipe_orchestrate_helper_bin)" apply-results "${ROOT}" "${DORKPIPE_ORCH_ROOT}" "${DORKPIPE_ORCH_PLAN_JSON}" "${DORKPIPE_ORCH_APPROVAL_MD}" "${DORKPIPE_ORCH_APPLY_DIR}/result.json"

printf '[dorkpipe] apply result ready at %s\n' "${DORKPIPE_ORCH_APPLY_DIR}/result.json" >&2
