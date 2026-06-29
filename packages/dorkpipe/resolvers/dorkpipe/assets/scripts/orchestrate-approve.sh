#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
decision="no"
if [[ "${DORKPIPE_ORCH_APPROVAL_MODE:-prompt}" == "auto-no" ]]; then
  decision="no"
elif [[ "${DORKPIPE_ORCH_APPROVAL_MODE:-prompt}" == "auto-yes" ]]; then
  decision="yes"
elif dockpipe_sdk prompt confirm \
  --id dorkpipe_orchestrate_approve \
  --title "Approve orchestration result?" \
  --message "Review ${DORKPIPE_ORCH_MERGE_DIR}/final.md and ${DORKPIPE_ORCH_VERIFY_DIR}/result.json. Approve this orchestration run for manual follow-up?" \
  --default no \
  --intent review \
  --automation-group docs-review; then
  decision="yes"
fi

cat > "${DORKPIPE_ORCH_APPROVAL_MD}" <<EOF
# Approval

- Approved: ${decision}
- Final synthesis: \`${DORKPIPE_ORCH_MERGE_DIR}/final.md\`
- Verify result: \`${DORKPIPE_ORCH_VERIFY_DIR}/result.json\`

This step records human disposition only.
EOF

printf '[dorkpipe] approval recorded at %s\n' "${DORKPIPE_ORCH_APPROVAL_MD}" >&2
