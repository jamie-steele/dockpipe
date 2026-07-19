#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"
requested_root="${ROOT:-${DOCKPIPE_WORKDIR:-}}"
eval "$(dockpipe sdk)"
dockpipe_sdk init-script

ROOT="${requested_root:-$(dockpipe_sdk get workdir)}"
artifact_root="${DORKPIPE_BACKLOG_ARTIFACT_ROOT:-$(dockpipe scope artifacts backlog-remote)}"
step_id="${DOCKPIPE_STEP_ID:-}"
unit="backlog.${step_id:-unknown}"
started_ms="$(dorkpipe_orchestrate_now_ms)"
helper_bin="$(dorkpipe_orchestrate_helper_bin)"

backlog_remote_fail() {
  local rc=$?
  dorkpipe_orchestrate_operation_fail "$unit" "$started_ms" "offline backlog ${step_id:-unknown} failed" \
    "artifact_root=$artifact_root"
  exit "$rc"
}
trap backlog_remote_fail ERR

if command -v cygpath >/dev/null 2>&1; then
  ROOT="$(cygpath -m "$ROOT")"
  artifact_root="$(cygpath -m "$artifact_root")"
fi

dorkpipe_orchestrate_operation_emit "$unit" start "" "artifact_root=$artifact_root"

case "$step_id" in
  inspect)
    MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-inspect \
      "$ROOT" \
      "${DORKPIPE_BACKLOG_TASK_INDEX:-docs/agents/task-index.yaml}" \
      "${DORKPIPE_BACKLOG_TASK_ID:-}" \
      "${DORKPIPE_BACKLOG_SLICE:-}" \
      "${DORKPIPE_BACKLOG_BASELINE:-}" \
      "$artifact_root"
    ;;
  compile)
    MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-compile \
      "$ROOT" \
      "$artifact_root" \
      "${DORKPIPE_BACKLOG_ENVIRONMENT_REF:-}" \
      "${DORKPIPE_BACKLOG_BRANCH_REF:-}" \
      "${DORKPIPE_BACKLOG_ALLOWED_PATHS_JSON:-[]}" \
      "${DORKPIPE_BACKLOG_HARD_BOUNDARIES_JSON:-[]}" \
      "${DORKPIPE_BACKLOG_REQUIRED_VALIDATION_JSON:-[]}" \
      "${DORKPIPE_BACKLOG_ROUTED_SOURCES_JSON:-[]}"
    ;;
  dispatch)
    fixture="${DORKPIPE_BACKLOG_DISPATCH_FIXTURE:-${DOCKPIPE_ASSETS_DIR}/fixtures/backlog-remote-dispatch.json}"
    if command -v cygpath >/dev/null 2>&1; then
      fixture="$(cygpath -m "$fixture")"
    fi
    MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-dispatch-fixture "$artifact_root" "$fixture"
    ;;
  *)
    echo "unsupported backlog.remote workflow step: ${step_id:-<empty>}" >&2
    exit 1
    ;;
esac

duration_ms="$(dorkpipe_orchestrate_operation_duration_ms "$started_ms")"
dorkpipe_orchestrate_operation_emit "$unit" done "$duration_ms" "artifact_root=$artifact_root" "adapter=fixture_only"
