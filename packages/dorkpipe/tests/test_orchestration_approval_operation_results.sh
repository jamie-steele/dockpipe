#!/usr/bin/env bash
set -euo pipefail
trap 'rc=$?; echo "test_orchestration_approval_operation_results failed at line ${LINENO}: ${BASH_COMMAND}" >&2; exit "$rc"' ERR

ROOT="$(git rev-parse --show-toplevel)"
export PATH="$ROOT/src/bin:$PATH"
export DOCKPIPE_SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
export DOCKPIPE_ASSETS_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets"
export DOCKPIPE_WORKFLOW_CONFIG="$ROOT/workflows/agent/docs.orchestrate/config.yml"
export DOCKPIPE_WORKFLOW_NAME="docs.orchestrate"
export DOCKPIPE_STEP_ID="approve"
export DORKPIPE_ORCH_WORKFLOW="test.docs.orchestrate.approval"
export DORKPIPE_ORCH_ROOT="${TMPDIR:-/tmp}/dorkpipe-orch-approval-${RANDOM}-${RANDOM}"

export DORKPIPE_ORCH_APPROVAL_MODE="auto-no"
auto_no_log="$DORKPIPE_ORCH_ROOT.auto-no.err"
bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-approve.sh" >/dev/null 2>"$auto_no_log"
grep -Fq -- "unit=orchestrate.approval status=start" "$auto_no_log"
grep -Fq -- "unit=orchestrate.approval status=done" "$auto_no_log"
grep -Fq -- "workflow=test.docs.orchestrate.approval" "$auto_no_log"
grep -Fq -- "mode=auto-no" "$auto_no_log"
grep -Fq -- "decision=review" "$auto_no_log"
grep -Fq -- "approved=no" "$auto_no_log"
auto_no_approval="$(sed -n 's/.* approval=\([^ ]*approval\.md\).*/\1/p' "$auto_no_log" | tail -1)"
[[ -n "$auto_no_approval" && -f "$auto_no_approval" ]]
grep -Fq -- "- Decision: review" "$auto_no_approval"
grep -Fq -- "- Approved: no" "$auto_no_approval"

export DORKPIPE_ORCH_APPROVAL_MODE="auto-yes"
auto_yes_log="$DORKPIPE_ORCH_ROOT.auto-yes.err"
bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-approve.sh" >/dev/null 2>"$auto_yes_log"
grep -Fq -- "unit=orchestrate.approval status=start" "$auto_yes_log"
grep -Fq -- "unit=orchestrate.approval status=done" "$auto_yes_log"
grep -Fq -- "workflow=test.docs.orchestrate.approval" "$auto_yes_log"
grep -Fq -- "mode=auto-yes" "$auto_yes_log"
grep -Fq -- "decision=approve" "$auto_yes_log"
grep -Fq -- "approved=yes" "$auto_yes_log"
auto_yes_approval="$(sed -n 's/.* approval=\([^ ]*approval\.md\).*/\1/p' "$auto_yes_log" | tail -1)"
[[ -n "$auto_yes_approval" && -f "$auto_yes_approval" ]]
grep -Fq -- "- Decision: approve" "$auto_yes_approval"
grep -Fq -- "- Approved: yes" "$auto_yes_approval"

echo "test_orchestration_approval_operation_results OK"
