#!/usr/bin/env bash
set -euo pipefail
trap 'rc=$?; echo "test_software_dev_workflow failed at line ${LINENO}: ${BASH_COMMAND}" >&2; exit "$rc"' ERR

REPO_ROOT="$(git rev-parse --show-toplevel)"
# shellcheck source=packages/dorkpipe/tests/lib/test-tools.sh
source "$REPO_ROOT/packages/dorkpipe/tests/lib/test-tools.sh"
dorkpipe_test_require_go "test_software_dev_workflow"
dorkpipe_test_init_go_cache "$REPO_ROOT"

tmp="$(dorkpipe_test_mktemp_dir "$REPO_ROOT")"
consumer="$tmp/consumer"
consumer_workflow_dir="$consumer/workflows/software-dev"
proof_output="$consumer/bin/.dockpipe/tmp/package-tests/software-dev-proof-output"
trap 'rm -rf "$tmp" "$proof_output"' EXIT
mkdir -p "$consumer_workflow_dir"
cp "$REPO_ROOT/packages/dorkpipe/tests/fixtures/software.dev/task-pack.yml" "$consumer_workflow_dir/task-pack.yml"
cp "$REPO_ROOT/packages/dorkpipe/tests/fixtures/software.dev/agents.yml" "$consumer_workflow_dir/agents.yml"
cp "$REPO_ROOT/packages/dorkpipe/tests/fixtures/software.dev/valid-proposal.yml" "$consumer_workflow_dir/valid-proposal.yml"
task_pack="workflows/software-dev/task-pack.yml"
valid_proposal="workflows/software-dev/valid-proposal.yml"
agents_file="workflows/software-dev/agents.yml"
rm -rf "$proof_output"
cp "$consumer/$task_pack" "$tmp/task-pack.before.yml"
cp "$consumer/$agents_file" "$tmp/agents.before.yml"

helper_bin="$tmp/orchestrate-helper"
(
  cd "$REPO_ROOT/packages/dorkpipe/lib"
  go build -o "$helper_bin" ./cmd/orchestrate-helper
)

workflow_config="$REPO_ROOT/packages/dorkpipe/workflows/software.dev/config.yml"
script_dir="$REPO_ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
export PATH="$REPO_ROOT/src/bin:$PATH"
export DOCKPIPE_SCRIPT_DIR="$script_dir"
export DOCKPIPE_ASSETS_DIR="$REPO_ROOT/packages/dorkpipe/resolvers/dorkpipe/assets"
export DOCKPIPE_WORKFLOW_CONFIG="$workflow_config"
export DOCKPIPE_WORKFLOW_NAME="software.dev"
export DORKPIPE_ORCH_WORKFLOW="test.software.dev"
export DORKPIPE_ORCH_HELPER_BIN="$helper_bin"
export DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS="$tmp/global-training.jsonl"
export DORKPIPE_ORCH_LIVE_MODELS="false"
export DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS="60000"
export DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS="20000"
export DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED="true"
export ROOT="$consumer"
export DORKPIPE_SOFTWARE_DEV_TASK_PACK="$task_pack"
cd "$consumer"

use_orch_root() {
  export DORKPIPE_ORCH_ROOT="${1:?orchestration root}"
  export DORKPIPE_ORCH_REQUEST_JSON="$DORKPIPE_ORCH_ROOT/request.json"
  export DORKPIPE_ORCH_PLAN_JSON="$DORKPIPE_ORCH_ROOT/plan.json"
  export DORKPIPE_ORCH_GRAPH_JSON="$DORKPIPE_ORCH_ROOT/task-graph.json"
  export DORKPIPE_ORCH_SHARED_DIR="$DORKPIPE_ORCH_ROOT/shared"
  export DORKPIPE_ORCH_TASKS_DIR="$DORKPIPE_ORCH_ROOT/tasks"
  export DORKPIPE_ORCH_MERGE_DIR="$DORKPIPE_ORCH_ROOT/merge"
  export DORKPIPE_ORCH_VERIFY_DIR="$DORKPIPE_ORCH_ROOT/verify"
  export DORKPIPE_ORCH_APPLY_DIR="$DORKPIPE_ORCH_ROOT/apply"
  export DORKPIPE_ORCH_LANES_DIR="$DORKPIPE_ORCH_ROOT/lanes"
  export DORKPIPE_ORCH_TRAINING_DIR="$DORKPIPE_ORCH_ROOT/training"
  export DORKPIPE_ORCH_APPROVAL_MD="$DORKPIPE_ORCH_ROOT/approval.md"
  export DORKPIPE_ORCH_CLOUD_USAGE_JSON="$DORKPIPE_ORCH_ROOT/cloud-usage.json"
  export DORKPIPE_ORCH_HALT_JSON="$DORKPIPE_ORCH_ROOT/halt.json"
}

static_root="$tmp/static-orchestrate"
use_orch_root "$static_root"
export DORKPIPE_SOFTWARE_DEV_TASK_PACK_STEP="static_pack"
export DORKPIPE_SOFTWARE_DEV_PLANNER_MODE="false"
export DORKPIPE_SOFTWARE_DEV_PLANNER_PROPOSAL_FIXTURE=""
export DOCKPIPE_STEP_ID="prepare"
bash "$script_dir/software-dev-orchestrate.sh"

grep -Fq '"id": "static_write"' "$static_root/task-graph.json"
grep -Fq '"id": "static_review"' "$static_root/task-graph.json"
if grep -Fq 'software_dev_planner' "$static_root/task-graph.json"; then
  echo "static task pack unexpectedly executed the planner bootstrap" >&2
  exit 1
fi
grep -Fq '"approval_required": true' "$static_root/plan.json"
grep -Fq '"publish": false' "$static_root/plan.json"
grep -Fq '"sync": false' "$static_root/plan.json"
grep -Fq '"target_root": "bin/.dockpipe/tmp/package-tests/software-dev-proof-output"' "$static_root/plan.json"
grep -Fq '"present": false' "$static_root/proposal/metadata.json"

deterministic_root="$tmp/static-deterministic"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" software-dev-compile \
  "$workflow_config" contract \
  "$consumer" "$task_pack" static_pack "$deterministic_root"
for path in request.json plan.json task-graph.json proposal/metadata.json; do
  cmp "$static_root/$path" "$deterministic_root/$path"
done
diff -r "$static_root/tasks" "$deterministic_root/tasks"

seed_root="$tmp/seed-orchestrate"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" software-dev-compile \
  "$workflow_config" contract \
  "$consumer" "$task_pack" seed_pack "$seed_root"
grep -Fq '"id": "software_dev_seed"' "$seed_root/task-graph.json"

runner="$tmp/fixture-runner.sh"
cat >"$runner" <<'RUNNER'
#!/usr/bin/env bash
set -euo pipefail
task_id="${1:?task id}"
task_dir="${DORKPIPE_ORCH_TASKS_DIR:?}/${task_id}"
mkdir -p "$task_dir/materialized"
case "$task_id" in
  static_write|plan_write)
    printf '%s\n' '# Required' '' 'Required fixture result.' >"$task_dir/materialized/required.md"
    printf '%s\n' '# Inferred' '' 'Additional inferred fixture result.' >"$task_dir/materialized/inferred.md"
    cat >"$task_dir/response.md" <<'EOF'
<dorkpipe:file path="required.md">
# Required

Required fixture result.
</dorkpipe:file>
<dorkpipe:file path="inferred.md">
# Inferred

Additional inferred fixture result.
</dorkpipe:file>
EOF
    ;;
  static_review|plan_review)
    printf '%s\n' '# Summary' '' 'Fixture summary result.' >"$task_dir/materialized/summary.md"
    cat >"$task_dir/response.md" <<'EOF'
<dorkpipe:file path="summary.md">
# Summary

Fixture summary result.
</dorkpipe:file>
EOF
    ;;
  *)
    echo "unexpected executable task: $task_id" >&2
    exit 1
    ;;
esac
printf '%s\n' "$task_id" >>"${DORKPIPE_SOFTWARE_DEV_EXECUTION_LOG:?}"
cat >"$task_dir/result.json" <<EOF
{
  "task_id": "$task_id",
  "base_task_id": "$task_id",
  "status": "ok",
  "provider": "fixture",
  "used_live_model": true,
  "summary": "Bounded fixture output for $task_id",
  "confidence": 0.95,
  "claims": [],
  "artifacts": ["tasks/$task_id/response.md"],
  "citations": [],
  "issues": [],
  "next_actions": []
}
EOF
RUNNER
chmod +x "$runner"

export DORKPIPE_SOFTWARE_DEV_EXECUTION_LOG="$tmp/static-executed.log"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" run-tasks "$static_root/task-graph.json" "$runner"
printf '%s\n' static_write static_review >"$tmp/static-expected.log"
cmp "$tmp/static-expected.log" "$DORKPIPE_SOFTWARE_DEV_EXECUTION_LOG"

export DORKPIPE_ORCH_APPROVAL_MODE="auto-no"
bash "$script_dir/orchestrate-merge-results.sh"
bash "$script_dir/orchestrate-verify-results.sh"
bash "$script_dir/orchestrate-approve.sh"
if bash "$script_dir/orchestrate-apply-results.sh" >"$tmp/unapproved.out" 2>"$tmp/unapproved.err"; then
  echo "software.dev apply succeeded without approval" >&2
  exit 1
fi

export DORKPIPE_ORCH_APPROVAL_MODE="auto-yes"
bash "$script_dir/orchestrate-approve.sh"
bash "$script_dir/orchestrate-apply-results.sh"
test -f "$proof_output/required.md"
test -f "$proof_output/summary.md"
test -f "$proof_output/inferred.md"
if find "$static_root" -mindepth 1 \( -iname '*publish*' -o -iname '*sync*' \) -print -quit | grep -q .; then
  echo "software.dev created publish or sync artifacts" >&2
  exit 1
fi

planner_root="$tmp/planner-orchestrate"
use_orch_root "$planner_root"
export DORKPIPE_SOFTWARE_DEV_TASK_PACK_STEP="planner_pack"
export DORKPIPE_SOFTWARE_DEV_PLANNER_MODE="true"
export DORKPIPE_SOFTWARE_DEV_PLANNER_PROPOSAL_FIXTURE="$valid_proposal"
export DOCKPIPE_STEP_ID="prepare"
bash "$script_dir/software-dev-orchestrate.sh"
grep -Fq '"id": "software_dev_planner"' "$planner_root/task-graph.json"
if grep -Fq '"id": "plan_write"' "$planner_root/task-graph.json"; then
  echo "proposed task appeared before planner proposal compilation" >&2
  exit 1
fi

export DOCKPIPE_STEP_ID="planner_run"
bash "$script_dir/software-dev-orchestrate.sh"
export DOCKPIPE_STEP_ID="compile"
bash "$script_dir/software-dev-orchestrate.sh"
grep -Fq '"id": "plan_write"' "$planner_root/task-graph.json"
grep -Fq '"id": "plan_review"' "$planner_root/task-graph.json"
if grep -Fq 'software_dev_planner' "$planner_root/task-graph.json"; then
  echo "planner bootstrap remained in the executable compiled graph" >&2
  exit 1
fi
grep -Fq '"present": true' "$planner_root/proposal/metadata.json"
grep -Fq '"selected_graph": true' "$planner_root/proposal/metadata.json"
test -f "$planner_root/proposal/raw.yaml"
test -f "$planner_root/proposal/normalized.json"

export DORKPIPE_SOFTWARE_DEV_EXECUTION_LOG="$tmp/planner-executed.log"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" run-tasks "$planner_root/task-graph.json" "$runner"
printf '%s\n' plan_write plan_review >"$tmp/planner-expected.log"
cmp "$tmp/planner-expected.log" "$DORKPIPE_SOFTWARE_DEV_EXECUTION_LOG"

mkdir -p "$planner_root/verify"
cat >"$planner_root/verify/result.json" <<'EOF'
{
  "status": "pass",
  "confidence": 0.91,
  "issues": [],
  "failure_class": "none",
  "value_bar": {
    "overall_score": 0.83,
    "verdict": "strong_orchestration_value"
  },
  "direct_worker_baseline": {
    "verdict": "orchestration_adds_value"
  }
}
EOF
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" software-dev-evaluate-promotion \
  "$consumer" "$task_pack" planner_pack "$planner_root"
candidate="$planner_root/proposal/promotion-candidate.json"
grep -Fq '"contract_version": "software.dev.promotion-candidate/v1"' "$candidate"
grep -Fq '"status": "eligible"' "$candidate"
grep -Fq '"task_pack_path": "workflows/software-dev/task-pack.yml"' "$candidate"
grep -Fq '"step_id": "planner_pack"' "$candidate"
grep -Fq '"path": "workflows/software-dev/agents.yml"' "$candidate"
grep -Fq '"role": "reusable fixture evidence writer"' "$candidate"
grep -Fq '"inferred.md"' "$candidate"
grep -Fq '"performed": false' "$candidate"
cp "$candidate" "$tmp/promotion-candidate.first.json"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" software-dev-evaluate-promotion \
  "$consumer" "$task_pack" planner_pack "$planner_root"
cmp "$tmp/promotion-candidate.first.json" "$candidate"
cmp "$tmp/task-pack.before.yml" "$consumer/$task_pack"
cmp "$tmp/agents.before.yml" "$consumer/$agents_file"
if find "$planner_root/proposal" -maxdepth 1 -name '.promotion-candidate-*.json' -print -quit | grep -q .; then
  echo "software.dev promotion evaluation left a temporary candidate" >&2
  exit 1
fi

MSYS2_ARG_CONV_EXCL='*' "$helper_bin" software-dev-build-promotion-patch \
  "$consumer" "$planner_root"
promotion_manifest="$planner_root/proposal/promotion-patch.json"
promotion_patch="$planner_root/proposal/promotion.patch"
grep -Fq '"contract_version": "software.dev.promotion-patch/v1"' "$promotion_manifest"
grep -Fq '"approval_required": true' "$promotion_manifest"
grep -Fq '"performed": false' "$promotion_manifest"
grep -Fq -- '--- a/workflows/software-dev/task-pack.yml' "$promotion_patch"
grep -Fq -- '--- a/workflows/software-dev/agents.yml' "$promotion_patch"
if grep '^+' "$promotion_patch" | grep -Eq 'plan_write|depends_on|max_cloud_tokens|worker:|target_root:|require_approval:|publish:|sync:'; then
  echo "software.dev promotion patch added a session-only or hard-authority field" >&2
  exit 1
fi
cmp "$tmp/task-pack.before.yml" "$consumer/$task_pack"
cmp "$tmp/agents.before.yml" "$consumer/$agents_file"

patch_digest="$(awk '/"patch": \{/{inside=1; next} inside && /"sha256":/{gsub(/[",]/, "", $2); print $2; exit}' "$promotion_manifest")"
task_pack_before_digest="$(grep -m1 '"before_sha256"' "$promotion_manifest" | sed -E 's/.*"before_sha256": "([^"]+)".*/\1/')"
agents_before_digest="$(grep '"before_sha256"' "$promotion_manifest" | sed -n '2s/.*"before_sha256": "\([^"]*\)".*/\1/p')"
promotion_approval="$planner_root/proposal/promotion-approval.json"
cat >"$promotion_approval" <<EOF
{
  "contract_version": "software.dev.promotion-approval/v1",
  "decision": "approve",
  "approved": true,
  "patch_sha256": "$patch_digest",
  "targets": [
    {
      "path": "$task_pack",
      "before_sha256": "$task_pack_before_digest"
    },
    {
      "path": "$agents_file",
      "before_sha256": "$agents_before_digest"
    }
  ]
}
EOF
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" software-dev-apply-promotion \
  "$consumer" "$planner_root" "$promotion_approval"
grep -Fq '"status": "applied"' "$planner_root/proposal/promotion-apply-result.json"
if cmp -s "$tmp/task-pack.before.yml" "$consumer/$task_pack" || cmp -s "$tmp/agents.before.yml" "$consumer/$agents_file"; then
  echo "approved promotion did not change both exact consumer targets" >&2
  exit 1
fi
grep -Fq 'Preserve reusable fixture guidance across future runs.' "$consumer/$task_pack"
grep -Fq 'Use the compiled planner graph only.' "$consumer/$task_pack"
grep -Fq 'Compile a bounded proposal before any proposed task runs.' "$consumer/$task_pack"
grep -Fq 'Planner software.dev fixture' "$consumer/$task_pack"
grep -Fq 'approve the governed planner fixture output' "$consumer/$task_pack"
grep -Fq 'reusable fixture evidence writer' "$consumer/$agents_file"
grep -Fq 'fixture writer' "$consumer/$agents_file"
grep -Fq 'inferred.md' "$consumer/$task_pack"

denied_consumer="$tmp/denied-consumer"
mkdir -p "$denied_consumer/workflows/software-dev"
cp "$tmp/task-pack.before.yml" "$denied_consumer/$task_pack"
cp "$tmp/agents.before.yml" "$denied_consumer/$agents_file"
cat >"$promotion_approval" <<EOF
{
  "contract_version": "software.dev.promotion-approval/v1",
  "decision": "deny",
  "approved": false,
  "patch_sha256": "$patch_digest",
  "targets": [
    {"path": "$task_pack", "before_sha256": "$task_pack_before_digest"},
    {"path": "$agents_file", "before_sha256": "$agents_before_digest"}
  ]
}
EOF
if MSYS2_ARG_CONV_EXCL='*' "$helper_bin" software-dev-apply-promotion \
  "$denied_consumer" "$planner_root" "$promotion_approval"; then
  echo "denied promotion unexpectedly applied" >&2
  exit 1
fi
cmp "$tmp/task-pack.before.yml" "$denied_consumer/$task_pack"
cmp "$tmp/agents.before.yml" "$denied_consumer/$agents_file"

stale_consumer="$tmp/stale-consumer"
mkdir -p "$stale_consumer/workflows/software-dev"
cp "$tmp/task-pack.before.yml" "$stale_consumer/$task_pack"
cp "$tmp/agents.before.yml" "$stale_consumer/$agents_file"
printf '%s\n' '# stale target' >>"$stale_consumer/$task_pack"
cp "$stale_consumer/$task_pack" "$tmp/stale-task-pack.before.yml"
cat >"$promotion_approval" <<EOF
{
  "contract_version": "software.dev.promotion-approval/v1",
  "decision": "approve",
  "approved": true,
  "patch_sha256": "$patch_digest",
  "targets": [
    {"path": "$task_pack", "before_sha256": "$task_pack_before_digest"},
    {"path": "$agents_file", "before_sha256": "$agents_before_digest"}
  ]
}
EOF
if MSYS2_ARG_CONV_EXCL='*' "$helper_bin" software-dev-apply-promotion \
  "$stale_consumer" "$planner_root" "$promotion_approval"; then
  echo "stale promotion unexpectedly applied" >&2
  exit 1
fi
cmp "$tmp/stale-task-pack.before.yml" "$stale_consumer/$task_pack"
cmp "$tmp/agents.before.yml" "$stale_consumer/$agents_file"

for invalid in malformed narrated structural authority-widening unknown-role invalid-dependency missing-output-floor; do
  invalid_root="$tmp/invalid-$invalid"
  mkdir -p "$invalid_root/tasks/sentinel"
  printf '%s\n' '{}' >"$invalid_root/task-graph.json"
  printf '%s\n' '{}' >"$invalid_root/tasks/sentinel/task.json"
  if MSYS2_ARG_CONV_EXCL='*' "$helper_bin" software-dev-compile \
    "$workflow_config" contract \
    "$consumer" "$task_pack" planner_pack "$invalid_root" "$REPO_ROOT/packages/dorkpipe/tests/fixtures/software.dev/invalid/$invalid.yml" \
    >"$tmp/$invalid.out" 2>"$tmp/$invalid.err"; then
    echo "invalid planner proposal unexpectedly compiled: $invalid" >&2
    exit 1
  fi
  test ! -e "$invalid_root/task-graph.json"
  test ! -e "$invalid_root/tasks"
  test -f "$invalid_root/proposal/rejected.txt"
done

echo "test_software_dev_workflow OK"
