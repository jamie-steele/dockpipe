#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
# shellcheck source=packages/dorkpipe/tests/lib/test-tools.sh
source "$ROOT/packages/dorkpipe/tests/lib/test-tools.sh"
dorkpipe_test_require_go "test_orchestration_lanes"
dorkpipe_test_init_go_cache "$ROOT"
export TMPDIR="${TMPDIR:-$(dorkpipe_test_tmp_root "$ROOT")}"
export PATH="$ROOT/src/bin:$PATH"
export DOCKPIPE_SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
export DOCKPIPE_ASSETS_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets"
export DOCKPIPE_WORKFLOW_CONFIG="$ROOT/workflows/agent/docs.orchestrate/config.yml"
export DOCKPIPE_WORKFLOW_NAME="docs.orchestrate"
export DOCKPIPE_STEP_ID="plan"
export DORKPIPE_ORCH_WORKFLOW="test.docs.orchestrate"
export DORKPIPE_ORCH_ROOT="${TMPDIR:-/tmp}/dorkpipe-orch-lanes-${RANDOM}-${RANDOM}"
export DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS="${TMPDIR:-/tmp}/dorkpipe-orch-global-metrics-${RANDOM}-${RANDOM}.jsonl"
export DORKPIPE_ORCH_LIVE_MODELS="false"
export DORKPIPE_ORCH_TRAINING_MODE="observe"
export DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS="1000"
export DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS="400"

plan_log="$DORKPIPE_ORCH_ROOT.plan.err"
bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-plan.sh" >/dev/null 2>"$plan_log"
grep -Fq -- "unit=orchestrate.plan status=start" "$plan_log"
grep -Fq -- "unit=orchestrate.plan status=done" "$plan_log"
grep -Fq -- "workflow=test.docs.orchestrate" "$plan_log"
grep -Fq -- "followup=false" "$plan_log"
bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-run-tasks.sh" >/dev/null

dorkpipe_test_assert "$ROOT" orchestration-lanes-initial "$DORKPIPE_ORCH_ROOT"

echo "test_orchestration_lanes OK"

dorkpipe_test_assert "$ROOT" orchestration-lanes-snapshot "$DORKPIPE_ORCH_ROOT"

export DORKPIPE_ORCH_FOLLOWUP_REQUEST="Tighten the package contract guidance without rewriting unrelated sections."
export DORKPIPE_ORCH_FOLLOWUP_GOAL="Repair only the package contract output and preserve untouched worker results."
export DORKPIPE_ORCH_FOLLOWUP_TASK_IDS="package_contracts"

followup_plan_log="$DORKPIPE_ORCH_ROOT.followup-plan.err"
bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-plan.sh" >/dev/null 2>"$followup_plan_log"
grep -Fq -- "unit=orchestrate.plan status=start" "$followup_plan_log"
grep -Fq -- "unit=orchestrate.plan status=done" "$followup_plan_log"
grep -Fq -- "workflow=test.docs.orchestrate" "$followup_plan_log"
grep -Fq -- "followup=true" "$followup_plan_log"
bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-run-tasks.sh" >/dev/null

dorkpipe_test_assert "$ROOT" orchestration-lanes-followup "$DORKPIPE_ORCH_ROOT"

unset DORKPIPE_ORCH_FOLLOWUP_REQUEST
unset DORKPIPE_ORCH_FOLLOWUP_GOAL
unset DORKPIPE_ORCH_FOLLOWUP_TASK_IDS

echo "test_orchestration_followup_reuse OK"

export DORKPIPE_ORCH_WORKFLOW="test.docs.orchestrate.force-codex"
export DORKPIPE_ORCH_ROOT="${TMPDIR:-/tmp}/dorkpipe-orch-force-codex-${RANDOM}-${RANDOM}"
export DORKPIPE_ORCH_FORCE_PROVIDER="codex"
export DORKPIPE_ORCH_CLOUD_LANES="true"

bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-plan.sh" >/dev/null

dorkpipe_test_assert "$ROOT" orchestration-force-codex "$DORKPIPE_ORCH_ROOT"

echo "test_orchestration_force_codex OK"

export DORKPIPE_ORCH_WORKFLOW="test.docs.orchestrate.brain-codex"
export DORKPIPE_ORCH_ROOT="${TMPDIR:-/tmp}/dorkpipe-orch-brain-codex-${RANDOM}-${RANDOM}"
export DORKPIPE_ORCH_FORCE_PROVIDER=""
export DORKPIPE_ORCH_BRAIN_PROVIDER="codex"
export DORKPIPE_ORCH_CLOUD_LANES="true"

bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-plan.sh" >/dev/null

dorkpipe_test_assert "$ROOT" orchestration-brain-codex "$DORKPIPE_ORCH_ROOT"

echo "test_orchestration_brain_provider_codex OK"

export DORKPIPE_ORCH_WORKFLOW="test.docs.orchestrate.compare"
export DORKPIPE_ORCH_ROOT="${TMPDIR:-/tmp}/dorkpipe-orch-compare-${RANDOM}-${RANDOM}"
export DORKPIPE_ORCH_FORCE_PROVIDER=""
export DORKPIPE_ORCH_BRAIN_PROVIDER=""
export DORKPIPE_ORCH_COMPARE_PROVIDERS="ollama,codex,claude"
export DORKPIPE_ORCH_COMPARE_SCOPE="auto"
export DORKPIPE_ORCH_CLOUD_LANES="true"
export DORKPIPE_ORCH_CODEX_MODEL="test-codex-model"
export DORKPIPE_ORCH_CLAUDE_MODEL="test-claude-model"
export DORKPIPE_ORCH_OLLAMA_MODEL="test-ollama-model"

bash "$DOCKPIPE_SCRIPT_DIR/orchestrate-plan.sh" >/dev/null

dorkpipe_test_assert "$ROOT" orchestration-compare "$DORKPIPE_ORCH_ROOT"

echo "test_orchestration_compare_lanes OK"

export DORKPIPE_ORCH_WORKFLOW="test.docs.orchestrate.cloud-usage"
export DORKPIPE_ORCH_ROOT="${TMPDIR:-/tmp}/dorkpipe-orch-cloud-usage-${RANDOM}-${RANDOM}"
export DORKPIPE_ORCH_COMPARE_PROVIDERS=""

# shellcheck source=/dev/null
source "$DOCKPIPE_SCRIPT_DIR/orchestrate-common.sh"
dorkpipe_orchestrate_init
dorkpipe_orchestrate_record_cloud_usage codex 100 50 1200
dorkpipe_orchestrate_record_cloud_usage codex 25 25 800
dorkpipe_orchestrate_record_cloud_usage claude 40 10 400

dorkpipe_test_assert "$ROOT" orchestration-cloud-usage "$DORKPIPE_ORCH_ROOT"

echo "test_orchestration_cloud_usage_metrics OK"
