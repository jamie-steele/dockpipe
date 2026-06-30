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

echo "test_dev_stack_gpu_policy OK"
