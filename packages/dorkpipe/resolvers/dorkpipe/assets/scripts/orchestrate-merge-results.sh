#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
op_started_ms="$(dorkpipe_orchestrate_now_ms)"
dorkpipe_orchestrate_operation_emit "orchestrate.merge" "start" "" "workflow=${DORKPIPE_ORCH_WORKFLOW}" "root=${DORKPIPE_ORCH_MERGE_DIR}"
mapfile -t result_paths < <(
  "$(dorkpipe_orchestrate_helper_bin)" merge-result-paths "${DORKPIPE_ORCH_GRAPH_JSON}" "${DORKPIPE_ORCH_TASKS_DIR}" all
)

mapfile -t main_result_paths < <(
  "$(dorkpipe_orchestrate_helper_bin)" merge-result-paths "${DORKPIPE_ORCH_GRAPH_JSON}" "${DORKPIPE_ORCH_TASKS_DIR}" main
)

mapfile -t planning_result_paths < <(
  "$(dorkpipe_orchestrate_helper_bin)" merge-result-paths "${DORKPIPE_ORCH_GRAPH_JSON}" "${DORKPIPE_ORCH_TASKS_DIR}" planning
)

missing=0
for path in "${result_paths[@]}"; do
  [[ -f "$path" ]] || missing=1
done
if [[ "$missing" -ne 0 ]]; then
  dorkpipe_orchestrate_operation_fail "orchestrate.merge" "${op_started_ms}" "missing worker result(s) for merge" "workflow=${DORKPIPE_ORCH_WORKFLOW}"
  echo "missing worker result(s) for merge" >&2
  exit 1
fi

eval "$("$(dorkpipe_orchestrate_helper_bin)" merge-plan-env "${DORKPIPE_ORCH_PLAN_JSON}")"

if ! "$(dorkpipe_orchestrate_helper_bin)" merge-build-result \
  "${DORKPIPE_ORCH_MERGE_DIR}/result.json" \
  "${main_result_paths[@]}" \
  --planning \
  "${planning_result_paths[@]}"; then
  dorkpipe_orchestrate_operation_fail "orchestrate.merge" "${op_started_ms}" "merge-build-result failed" "workflow=${DORKPIPE_ORCH_WORKFLOW}"
  exit 1
fi

if ! "$(dorkpipe_orchestrate_helper_bin)" merge-render-final "${DORKPIPE_ORCH_MERGE_DIR}/result.json" "${DORKPIPE_ORCH_MERGE_DIR}/final.md" "${DORKPIPE_ORCH_TASKS_DIR}"; then
  dorkpipe_orchestrate_operation_fail "orchestrate.merge" "${op_started_ms}" "merge-render-final failed" "workflow=${DORKPIPE_ORCH_WORKFLOW}"
  exit 1
fi

dorkpipe_orchestrate_operation_emit "orchestrate.merge" "done" "$(dorkpipe_orchestrate_operation_duration_ms "${op_started_ms}")" "workflow=${DORKPIPE_ORCH_WORKFLOW}" "result=${DORKPIPE_ORCH_MERGE_DIR}/result.json"
