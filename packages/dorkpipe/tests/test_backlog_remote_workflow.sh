#!/usr/bin/env bash
set -euo pipefail
trap 'rc=$?; echo "test_backlog_remote_workflow failed at line ${LINENO}: ${BASH_COMMAND}" >&2; exit "$rc"' ERR

REPO_ROOT="$(git rev-parse --show-toplevel)"
# shellcheck source=packages/dorkpipe/tests/lib/test-tools.sh
source "$REPO_ROOT/packages/dorkpipe/tests/lib/test-tools.sh"
dorkpipe_test_require_go "test_backlog_remote_workflow"
dorkpipe_test_init_go_cache "$REPO_ROOT"

tmp="$(dorkpipe_test_mktemp_dir "$REPO_ROOT")"
consumer="$tmp/consumer"
pristine="$tmp/pristine"
artifact_root="$tmp/artifacts"
second_root="$tmp/artifacts-second"
fixture_root="$REPO_ROOT/packages/dorkpipe/tests/fixtures/backlog.remote"
compatibility_fixture="$REPO_ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/fixtures/backlog-remote-codex-cli"
helper_bin="$tmp/orchestrate-helper"
invocation_log="$tmp/forbidden-invocations.log"
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$consumer" "$tmp/fake-bin"
cp -R "$fixture_root/consumer/." "$consumer/"
cp -R "$consumer" "$pristine"
(
  cd "$REPO_ROOT/packages/dorkpipe/lib"
  go build -o "$helper_bin" ./cmd/orchestrate-helper
)

cat >"$tmp/fake-bin/forbidden-tool" <<'TOOL'
#!/usr/bin/env bash
printf '%s\n' "$(basename "$0") $*" >>"${DORKPIPE_BACKLOG_FORBIDDEN_LOG:?}"
exit 97
TOOL
chmod +x "$tmp/fake-bin/forbidden-tool"
for tool in codex curl git ssh; do
  cp "$tmp/fake-bin/forbidden-tool" "$tmp/fake-bin/$tool"
done

export PATH="$tmp/fake-bin:$REPO_ROOT/src/bin:$PATH"
export DORKPIPE_BACKLOG_FORBIDDEN_LOG="$invocation_log"
export DOCKPIPE_SCRIPT_DIR="$REPO_ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
export DOCKPIPE_ASSETS_DIR="$REPO_ROOT/packages/dorkpipe/resolvers/dorkpipe/assets"
export DOCKPIPE_WORKFLOW_CONFIG="$REPO_ROOT/packages/dorkpipe/workflows/backlog.remote/config.yml"
export DOCKPIPE_WORKFLOW_NAME="backlog.remote"
export DORKPIPE_ORCH_HELPER_BIN="$helper_bin"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$artifact_root"
export DORKPIPE_BACKLOG_TASK_INDEX="docs/agents/task-index.yaml"
export DORKPIPE_BACKLOG_TASK_ID="TASK-015"
export DORKPIPE_BACKLOG_SLICE="Implement only the bounded offline fixture dispatch slice."
export DORKPIPE_BACKLOG_BASELINE="0123456789abcdef0123456789abcdef01234567"
export DORKPIPE_BACKLOG_ENVIRONMENT_REF="fixture-environment"
export DORKPIPE_BACKLOG_BRANCH_REF="js/dev"
export DORKPIPE_BACKLOG_ALLOWED_PATHS_JSON='["packages/dorkpipe","docs/agents/tasks/backlog-driven-remote-tasks.md"]'
export DORKPIPE_BACKLOG_HARD_BOUNDARIES_JSON='["No live provider invocation","No apply, commit, push, or publication"]'
export DORKPIPE_BACKLOG_REQUIRED_VALIDATION_JSON='["go test ./packages/dorkpipe/lib/orchestrationhelper"]'
export DORKPIPE_BACKLOG_ROUTED_SOURCES_JSON='["docs/agents/packages/package-authoring.md","docs/agents/workflows/yaml-workflows.md"]'
export DORKPIPE_BACKLOG_COMPATIBILITY_FIXTURE="$compatibility_fixture"
export DORKPIPE_BACKLOG_DISPATCH_FIXTURE="$fixture_root/dispatch.json"
export DORKPIPE_BACKLOG_COMPLETION_FIXTURE="$fixture_root/completion-candidate.json"
export ROOT="$consumer"

log="$tmp/workflow.err"
for step in inspect compile compatibility dispatch completion_candidate; do
  export DOCKPIPE_STEP_ID="$step"
  bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>>"$log"
done

for step in inspect compile compatibility dispatch completion_candidate; do
  grep -Fq "unit=backlog.$step status=start" "$log"
  grep -Fq "unit=backlog.$step status=done" "$log"
done
grep -Fq "unit=backlog.compatibility status=done" "$log"
grep -Fq "compatibility=unsupported" "$log"
grep -Fq "reason=machine_readable_submission_receipt_not_documented" "$log"
grep -Fq "live_submission=false" "$log"
grep -Fq "unit=backlog.completion_candidate status=done" "$log"
grep -Fq "authoritative_state=completion_candidate" "$log"
grep -Fq "ready_for_review=false" "$log"
grep -Fq "terminal_claim_trusted=false" "$log"
for name in backlog-selection.json remote-request.md remote-request.json remote-adapter-compatibility.json remote-task.json completion-candidate.json; do
  test -f "$artifact_root/$name"
done
grep -Fq '"status": "selected"' "$artifact_root/backlog-selection.json"
grep -Fq '"contract_version": "dorkpipe.remote-request/v1"' "$artifact_root/remote-request.json"
grep -Fq '"adapter_mode": "fixture_only"' "$artifact_root/remote-request.json"
grep -Fq '"live_provider": false' "$artifact_root/remote-request.json"
grep -Fq '"contract_version": "dorkpipe.remote-adapter-compatibility/v1"' "$artifact_root/remote-adapter-compatibility.json"
grep -Fq '"version": "codex-cli 0.144.1"' "$artifact_root/remote-adapter-compatibility.json"
grep -Fq '"status": "unsupported"' "$artifact_root/remote-adapter-compatibility.json"
grep -Fq '"machine_readable_documented": false' "$artifact_root/remote-adapter-compatibility.json"
grep -Fq '"stable_opaque_task_id_recoverable": false' "$artifact_root/remote-adapter-compatibility.json"
grep -Fq '"live_submission_enabled": false' "$artifact_root/remote-adapter-compatibility.json"
grep -Fq '"provider_invoked": false' "$artifact_root/remote-task.json"
grep -Fq '"remote_task_id": "remote_fixture_task_015"' "$artifact_root/remote-task.json"
grep -Fq '"compatibility_fingerprint": "sha256:' "$artifact_root/remote-task.json"
grep -Fq '"contract_version": "dorkpipe.remote-completion-candidate/v1"' "$artifact_root/completion-candidate.json"
grep -Fq '"state": "completion_candidate"' "$artifact_root/completion-candidate.json"
grep -Fq '"candidate_id": "completion_fixture_candidate_015"' "$artifact_root/completion-candidate.json"
grep -Fq '"replay_identity": "completion_fixture_replay_015"' "$artifact_root/completion-candidate.json"
grep -Fq '"terminal_claim_trusted": false' "$artifact_root/completion-candidate.json"
grep -Fq '"ready_for_review": false' "$artifact_root/completion-candidate.json"
if grep -Fq '"ready_for_review": true' "$artifact_root/completion-candidate.json"; then
  echo "completion candidate unexpectedly enabled ready_for_review" >&2
  exit 1
fi
test ! -e "$invocation_log"
diff -r "$pristine" "$consumer"

MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-inspect \
  "$consumer" docs/agents/task-index.yaml TASK-015 \
  "$DORKPIPE_BACKLOG_SLICE" "$DORKPIPE_BACKLOG_BASELINE" "$second_root"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-compile \
  "$consumer" "$second_root" "$DORKPIPE_BACKLOG_ENVIRONMENT_REF" "$DORKPIPE_BACKLOG_BRANCH_REF" \
  "$DORKPIPE_BACKLOG_ALLOWED_PATHS_JSON" "$DORKPIPE_BACKLOG_HARD_BOUNDARIES_JSON" \
  "$DORKPIPE_BACKLOG_REQUIRED_VALIDATION_JSON" "$DORKPIPE_BACKLOG_ROUTED_SOURCES_JSON"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-compatibility-preflight "$second_root" "$compatibility_fixture"
test ! -e "$second_root/remote-task.json"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-dispatch-fixture "$second_root" "$DORKPIPE_BACKLOG_DISPATCH_FIXTURE"
for name in backlog-selection.json remote-request.md remote-request.json remote-adapter-compatibility.json remote-task.json; do
  if ! cmp "$artifact_root/$name" "$second_root/$name"; then
    diff -u "$artifact_root/$name" "$second_root/$name" >&2 || true
    exit 1
  fi
done

malformed_candidate_root="$tmp/malformed-candidate-artifacts"
tampered_dispatch_root="$tmp/tampered-dispatch-artifacts"
cp -R "$second_root" "$malformed_candidate_root"
cp -R "$second_root" "$tampered_dispatch_root"

malformed_root="$tmp/malformed-compatibility"
malformed_fixture="$tmp/malformed-fixture"
mkdir -p "$malformed_fixture"
printf '{}\n' >"$malformed_fixture/contract.json"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-inspect \
  "$consumer" docs/agents/task-index.yaml TASK-015 \
  "$DORKPIPE_BACKLOG_SLICE" "$DORKPIPE_BACKLOG_BASELINE" "$malformed_root"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-compile \
  "$consumer" "$malformed_root" "$DORKPIPE_BACKLOG_ENVIRONMENT_REF" "$DORKPIPE_BACKLOG_BRANCH_REF" \
  "$DORKPIPE_BACKLOG_ALLOWED_PATHS_JSON" "$DORKPIPE_BACKLOG_HARD_BOUNDARIES_JSON" \
  "$DORKPIPE_BACKLOG_REQUIRED_VALIDATION_JSON" "$DORKPIPE_BACKLOG_ROUTED_SOURCES_JSON"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$malformed_root"
export DORKPIPE_BACKLOG_COMPATIBILITY_FIXTURE="$malformed_fixture"
export DOCKPIPE_STEP_ID="compatibility"
if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$tmp/malformed.err"; then
  echo "malformed compatibility contract unexpectedly passed" >&2
  exit 1
fi
grep -Fq 'unit=backlog.compatibility status=start' "$tmp/malformed.err"
grep -Fq 'unit=backlog.compatibility status=fail' "$tmp/malformed.err"
grep -Fq '"status": "error"' "$malformed_root/remote-adapter-compatibility.json"
grep -Fq '"reason_code": "invalid_compatibility_fixture"' "$malformed_root/remote-adapter-compatibility.json"
test ! -e "$malformed_root/remote-task.json"
export DORKPIPE_BACKLOG_COMPATIBILITY_FIXTURE="$compatibility_fixture"

malformed_candidate_fixture="$tmp/malformed-completion-candidate.json"
printf '{"unexpected":true}\n' >"$malformed_candidate_fixture"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$malformed_candidate_root"
export DORKPIPE_BACKLOG_COMPLETION_FIXTURE="$malformed_candidate_fixture"
export DOCKPIPE_STEP_ID="completion_candidate"
if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$tmp/malformed-candidate.err"; then
  echo "malformed completion candidate unexpectedly passed" >&2
  exit 1
fi
grep -Fq 'unit=backlog.completion_candidate status=start' "$tmp/malformed-candidate.err"
grep -Fq 'unit=backlog.completion_candidate status=fail' "$tmp/malformed-candidate.err"
if ! grep -Fq 'completion_candidate_fixture_malformed:' "$tmp/malformed-candidate.err"; then
  cat "$tmp/malformed-candidate.err" >&2
  exit 1
fi
grep -Fq 'reason_code=completion_candidate_fixture_malformed' "$tmp/malformed-candidate.err"
grep -Fq 'reason_code=completion_candidate_fixture_malformed' "$tmp/malformed-candidate.err"
test ! -e "$malformed_candidate_root/completion-candidate.json"

sed -i 's/remote_fixture_task_015/remote_fixture_task_tampered/' "$tampered_dispatch_root/remote-task.json"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$tampered_dispatch_root"
export DORKPIPE_BACKLOG_COMPLETION_FIXTURE="$fixture_root/completion-candidate.json"
if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$tmp/tampered-candidate.err"; then
  echo "tampered immutable dispatch unexpectedly ingested a completion candidate" >&2
  exit 1
fi
grep -Fq 'unit=backlog.completion_candidate status=start' "$tmp/tampered-candidate.err"
grep -Fq 'unit=backlog.completion_candidate status=fail' "$tmp/tampered-candidate.err"
grep -Fq 'completion_candidate_dispatch_invalid:' "$tmp/tampered-candidate.err"
grep -Fq 'reason_code=completion_candidate_dispatch_invalid' "$tmp/tampered-candidate.err"
grep -Fq 'reason_code=completion_candidate_dispatch_invalid' "$tmp/tampered-candidate.err"
test ! -e "$tampered_dispatch_root/completion-candidate.json"

rejected_root="$tmp/rejected"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$rejected_root"
export DORKPIPE_BACKLOG_TASK_ID="TASK-999"
export DOCKPIPE_STEP_ID="inspect"
if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$tmp/rejected.err"; then
  echo "unknown backlog task unexpectedly inspected" >&2
  exit 1
fi
grep -Fq 'unit=backlog.inspect status=start' "$tmp/rejected.err"
grep -Fq 'unit=backlog.inspect status=fail' "$tmp/rejected.err"
grep -Fq '"code": "unknown_task_id"' "$rejected_root/backlog-selection.json"
for name in remote-request.md remote-request.json remote-task.json; do
  test ! -e "$rejected_root/$name"
done

rm -rf "$consumer"
export DORKPIPE_BACKLOG_TASK_ID="TASK-015"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$artifact_root"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-followup "$artifact_root" >"$tmp/followup.json"
grep -Fq '"contract_version": "dorkpipe.remote-followup/v1"' "$tmp/followup.json"
grep -Fq '"remote_task_id": "remote_fixture_task_015"' "$tmp/followup.json"

MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-ingest-completion-candidate "$second_root" "$fixture_root/completion-candidate.json"
cmp "$artifact_root/completion-candidate.json" "$second_root/completion-candidate.json"

cp "$artifact_root/completion-candidate.json" "$tmp/accepted-completion-candidate.json"
cp "$artifact_root/remote-task.json" "$tmp/accepted-remote-task.json"
export DORKPIPE_BACKLOG_COMPLETION_FIXTURE="$fixture_root/completion-candidate.json"
export DOCKPIPE_STEP_ID="completion_candidate"
if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$tmp/duplicate-candidate.err"; then
  echo "duplicate completion candidate unexpectedly passed" >&2
  exit 1
fi
grep -Fq 'unit=backlog.completion_candidate status=start' "$tmp/duplicate-candidate.err"
grep -Fq 'unit=backlog.completion_candidate status=fail' "$tmp/duplicate-candidate.err"
grep -Fq 'completion_candidate_duplicate:' "$tmp/duplicate-candidate.err"
grep -Fq 'reason_code=completion_candidate_duplicate' "$tmp/duplicate-candidate.err"
grep -Fq 'reason_code=completion_candidate_duplicate' "$tmp/duplicate-candidate.err"
cmp "$tmp/accepted-completion-candidate.json" "$artifact_root/completion-candidate.json"
cmp "$tmp/accepted-remote-task.json" "$artifact_root/remote-task.json"
test ! -e "$invocation_log"

if find "$artifact_root" -mindepth 1 \( -iname '*status*' -o -iname '*diff*' -o -iname '*result*' -o -iname '*apply*' -o -iname '*commit*' -o -iname '*push*' -o -iname '*publish*' \) -print -quit | grep -q .; then
  echo "fixture slice created a forbidden lifecycle artifact" >&2
  exit 1
fi

echo "test_backlog_remote_workflow OK"
