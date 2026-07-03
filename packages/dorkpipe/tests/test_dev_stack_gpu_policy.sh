#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
LIB="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts/dev-stack-lib.sh"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

assert_file_equals() {
  local path="$1"
  local expected="$2"
  local got
  got="$(tr -d '\r\n' < "$path")"
  if [[ "$got" != "$expected" ]]; then
    echo "test_dev_stack_gpu_policy: expected $path to contain $expected, got $got" >&2
    exit 1
  fi
}

reset_gpu_env() {
  unset DOCKPIPE_WORKFLOW_NAME DORKPIPE_DEV_STACK_GPU DORKPIPE_DEV_STACK_GPU_SETUP \
    DORKPIPE_DEV_STACK_GPU_ON_FAILURE DORKPIPE_DEV_STACK_INTERACTIVE
  export DORKPIPE_DEV_STACK_STATE_DIR="$TMPDIR/state"
  export ROOT="$ROOT"
  rm -rf "$DORKPIPE_DEV_STACK_STATE_DIR"
  mkdir -p "$DORKPIPE_DEV_STACK_STATE_DIR"
}

source_lib() {
  # shellcheck source=/dev/null
  source "$LIB"
}

reset_gpu_env
source_lib
setup_attempts=0
prompt_calls=0
dockpipe_sdk() { prompt_calls=$((prompt_calls + 1)); return 1; }
dorkpipe_stack_detect_nvidia_gpu() { return 0; }
dorkpipe_stack_docker_supports_nvidia_gpu() { return 1; }
dorkpipe_stack_try_enable_docker_gpu_access() { setup_attempts=$((setup_attempts + 1)); return 0; }
export DOCKPIPE_WORKFLOW_NAME="stack-demo"
export DORKPIPE_DEV_STACK_GPU="auto"
dorkpipe_stack_configure_gpu
assert_file_equals "$(dorkpipe_stack_gpu_mode_file)" "cpu"
if [[ -e "$(dorkpipe_stack_gpu_compose_file)" ]]; then
  echo "test_dev_stack_gpu_policy: workflow auto path should not leave a GPU override file when Docker GPU is unavailable" >&2
  exit 1
fi
if [[ "$setup_attempts" -ne 0 || "$prompt_calls" -ne 0 ]]; then
  echo "test_dev_stack_gpu_policy: workflow auto path should not attempt setup or prompt by default" >&2
  exit 1
fi

reset_gpu_env
source_lib
setup_attempts=0
dorkpipe_stack_detect_nvidia_gpu() { return 0; }
dorkpipe_stack_docker_supports_nvidia_gpu() { return 1; }
dorkpipe_stack_try_enable_docker_gpu_access() { setup_attempts=$((setup_attempts + 1)); return 1; }
export DOCKPIPE_WORKFLOW_NAME="stack-demo"
export DORKPIPE_DEV_STACK_GPU="auto"
export DORKPIPE_DEV_STACK_GPU_SETUP="auto"
export DORKPIPE_DEV_STACK_GPU_ON_FAILURE="fail"
if dorkpipe_stack_configure_gpu; then
  echo "test_dev_stack_gpu_policy: explicit setup=auto + on_failure=fail should fail when GPU setup cannot complete" >&2
  exit 1
fi
if [[ "$setup_attempts" -ne 1 ]]; then
  echo "test_dev_stack_gpu_policy: expected one automatic GPU setup attempt, got $setup_attempts" >&2
  exit 1
fi

reset_gpu_env
source_lib
prompt_calls=0
dorkpipe_stack_detect_nvidia_gpu() { return 0; }
dorkpipe_stack_docker_supports_nvidia_gpu() { return 1; }
dorkpipe_stack_prompt_for_gpu_recovery() { prompt_calls=$((prompt_calls + 1)); return 30; }
export DORKPIPE_DEV_STACK_GPU="auto"
export DORKPIPE_DEV_STACK_INTERACTIVE="1"
dorkpipe_stack_configure_gpu
assert_file_equals "$(dorkpipe_stack_gpu_mode_file)" "cpu"
if [[ "$prompt_calls" -ne 1 ]]; then
  echo "test_dev_stack_gpu_policy: manual interactive auto path should prompt once before CPU fallback" >&2
  exit 1
fi

reset_gpu_env
source_lib
setup_attempts=0
prompt_calls=0
dockpipe_sdk() { prompt_calls=$((prompt_calls + 1)); return 1; }
dorkpipe_stack_detect_nvidia_gpu() { return 0; }
dorkpipe_stack_docker_supports_nvidia_gpu() { return 1; }
dorkpipe_stack_try_enable_docker_gpu_access() { setup_attempts=$((setup_attempts + 1)); return 0; }
export DOCKPIPE_WORKFLOW_NAME="stack-demo"
export DORKPIPE_DEV_STACK_GPU="nvidia"
if dorkpipe_stack_configure_gpu; then
  echo "test_dev_stack_gpu_policy: required NVIDIA mode should fail when Docker GPU access is unavailable" >&2
  exit 1
fi
if [[ "$setup_attempts" -ne 0 || "$prompt_calls" -ne 0 ]]; then
  echo "test_dev_stack_gpu_policy: required NVIDIA workflow path should fail without setup attempts or prompts by default" >&2
  exit 1
fi

reset_gpu_env
source_lib
fake_dockpipe="$TMPDIR/dockpipe"
cat > "$fake_dockpipe" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "result" ]]; then
  shift
  unit=""
  status=""
  duration_ms=""
  fields=()
  error=""
  while (($#)); do
    case "${1:-}" in
      --unit) unit="${2:-}"; shift 2 ;;
      --status) status="${2:-}"; shift 2 ;;
      --duration-ms) duration_ms="${2:-}"; shift 2 ;;
      --id) fields+=("${2:-}"); shift 2 ;;
      --error) error="${2:-}"; shift 2 ;;
      *) shift ;;
    esac
  done
  printf '[dockpipe] unit=%s status=%s' "${unit}" "${status}" >&2
  if [[ -n "${duration_ms}" && "${status}" != "start" ]]; then
    printf ' duration_ms=%s' "${duration_ms}" >&2
  fi
  for field in "${fields[@]}"; do
    [[ -n "${field}" ]] && printf ' %s' "${field}" >&2
  done
  if [[ -n "${error}" ]]; then
    printf ' error="%s"' "${error}" >&2
  fi
  printf '\n' >&2
  exit 0
fi
exit 1
SH
chmod +x "$fake_dockpipe"
export DOCKPIPE_BIN="$fake_dockpipe"
dorkpipe_stack_run_logged "docker compose down" "$TMPDIR/down.log" bash -c 'printf ok' 2>"$TMPDIR/run-logged-ok.err"
grep -Fq -- "[dockpipe] unit=devstack.docker-compose-down status=start" "$TMPDIR/run-logged-ok.err"
grep -Fq -- "[dockpipe] unit=devstack.docker-compose-down status=done" "$TMPDIR/run-logged-ok.err"
grep -Fq -- "log=$TMPDIR/down.log" "$TMPDIR/run-logged-ok.err"
if dorkpipe_stack_run_logged "docker compose fail" "$TMPDIR/fail.log" bash -c 'exit 7' 2>"$TMPDIR/run-logged-fail.err"; then
  echo "test_dev_stack_gpu_policy: expected failing logged command to return non-zero" >&2
  exit 1
fi
grep -Fq -- "[dockpipe] unit=devstack.docker-compose-fail status=fail" "$TMPDIR/run-logged-fail.err"
grep -Fq -- 'error="command exited 7"' "$TMPDIR/run-logged-fail.err"
unset DOCKPIPE_BIN

echo "test_dev_stack_gpu_policy OK"
