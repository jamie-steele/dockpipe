#!/usr/bin/env bash
set -euo pipefail
trap 'rc=$?; echo "test_orchestration_verify_status failed at line ${LINENO}: ${BASH_COMMAND}" >&2; exit "$rc"' ERR

ROOT="$(git rev-parse --show-toplevel)"
# shellcheck source=packages/dorkpipe/tests/lib/test-tools.sh
source "$ROOT/packages/dorkpipe/tests/lib/test-tools.sh"
dorkpipe_test_require_go "test_orchestration_verify_status"
dorkpipe_test_init_go_cache "$ROOT"
export TMPDIR="${TMPDIR:-$(dorkpipe_test_tmp_root "$ROOT")}"
export PATH="$ROOT/src/bin:$PATH"
export DOCKPIPE_SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
export DOCKPIPE_ASSETS_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets"
export DOCKPIPE_WORKFLOW_CONFIG="$ROOT/workflows/agent/docs.orchestrate/config.yml"
export DOCKPIPE_WORKFLOW_NAME="docs.orchestrate"
export DOCKPIPE_STEP_ID="verify"
export DORKPIPE_ORCH_WORKFLOW="test.docs.orchestrate.verify-status"
export DORKPIPE_ORCH_ROOT="${TMPDIR:-/tmp}/dorkpipe-orch-verify-status-${RANDOM}-${RANDOM}"

# shellcheck source=/dev/null
source "$DOCKPIPE_SCRIPT_DIR/orchestrate-common.sh"
dorkpipe_orchestrate_init

cat > "$DORKPIPE_ORCH_PLAN_JSON" <<'EOF'
{"apply":{"outputs":[{"source":"merge/index.md","path":"docs/index.md"}]}}
EOF
cat > "$DORKPIPE_ORCH_GRAPH_JSON" <<'EOF'
{"tasks":[]}
EOF
cat > "$DORKPIPE_ORCH_MERGE_DIR/result.json" <<'EOF'
{"tasks":[{"task_id":"writer","provider_actual":"codex","used_live_model":true,"confidence":0.8}],"average_confidence":0.8}
EOF
mkdir -p "$DORKPIPE_ORCH_TASKS_DIR/writer"
printf '%s\n' '- done' > "$DORKPIPE_ORCH_TASKS_DIR/writer/response.md"
printf '%s\n' '[Missing](./missing.md)' > "$DORKPIPE_ORCH_MERGE_DIR/index.md"
cat > "$DORKPIPE_ORCH_HALT_JSON" <<'EOF'
{"halted":true,"reason":"test budget halt"}
EOF

bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-verify-results.sh" >/dev/null
grep -Fq -- '"status": "fail"' "$DORKPIPE_ORCH_VERIFY_DIR/result.json"
grep -Fq -- 'markdown link target is missing' "$DORKPIPE_ORCH_VERIFY_DIR/result.json"

echo "test_orchestration_verify_status OK"
