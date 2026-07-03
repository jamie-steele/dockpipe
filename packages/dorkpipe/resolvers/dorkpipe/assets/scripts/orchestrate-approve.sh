#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
decision="review"
if [[ "${DORKPIPE_ORCH_APPROVAL_MODE:-prompt}" == "auto-no" ]]; then
  decision="review"
elif [[ "${DORKPIPE_ORCH_APPROVAL_MODE:-prompt}" == "auto-yes" ]]; then
  decision="approve"
else
  decision="$(dockpipe_sdk prompt choice \
  --id dorkpipe_orchestrate_approve \
  --title "Approve orchestration result?" \
  --message "Review ${DORKPIPE_ORCH_MERGE_DIR}/final.md and ${DORKPIPE_ORCH_VERIFY_DIR}/result.json. Choose whether this orchestration result is ready for manual follow-up." \
  --option review \
  --option approve \
  --default review \
  --intent review \
  --automation-group docs-review)"
fi

approved="no"
if [[ "${decision}" == "approve" ]]; then
  approved="yes"
fi

cat > "${DORKPIPE_ORCH_APPROVAL_MD}" <<EOF
# Approval

- Decision: ${decision}
- Approved: ${approved}
- Final synthesis: \`${DORKPIPE_ORCH_MERGE_DIR}/final.md\`
- Verify result: \`${DORKPIPE_ORCH_VERIFY_DIR}/result.json\`

This step records human disposition only.
EOF

printf '[dorkpipe] approval recorded at %s\n' "${DORKPIPE_ORCH_APPROVAL_MD}" >&2
