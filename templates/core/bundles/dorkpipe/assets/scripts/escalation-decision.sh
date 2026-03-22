#!/usr/bin/env bash
# Optional hook: read calibrated threshold from env; exit 0 = escalate, 1 = skip.
# Engine already gates escalation; use this for extra policy or notifications.
set -euo pipefail
TH="${DORKPIPE_ESCALATE_BELOW:-0.75}"
echo "escalation-decision: threshold=$TH (engine uses spec policy; this hook is optional)"
exit 0
