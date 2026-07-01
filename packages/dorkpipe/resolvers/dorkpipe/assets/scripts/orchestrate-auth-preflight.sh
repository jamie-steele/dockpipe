#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
[[ -d "${DORKPIPE_ORCH_TASKS_DIR}" ]] || {
  echo "missing task directory: ${DORKPIPE_ORCH_TASKS_DIR}" >&2
  exit 1
}

mapfile -t required_providers < <("$(dorkpipe_orchestrate_helper_bin)" required-auth-providers "${DORKPIPE_ORCH_TASKS_DIR}")
if ((${#required_providers[@]} == 0)); then
  printf '[dorkpipe] auth preflight: no required cloud provider auth checks\n' >&2
  exit 0
fi

for provider in "${required_providers[@]}"; do
  [[ -n "${provider}" ]] || continue
  printf '[dorkpipe] auth preflight: checking %s\n' "${provider}" >&2
  dorkpipe_orchestrate_auth_preflight "${provider}"
done

printf '[dorkpipe] auth preflight: all required provider auth checks passed\n' >&2
