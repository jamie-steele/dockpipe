#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/lib/dorkpipe-cli.sh"

ROOT="${DOCKPIPE_SOURCE_ROOT:-${DOCKPIPE_WORKDIR:-$(pwd)}}"

if [[ -z "${GOCACHE:-}" ]]; then
  GOCACHE="$(dockpipe scope --package dorkpipe build go-cache --workdir "$ROOT")"
  export GOCACHE
fi
if [[ -z "${GOTMPDIR:-}" ]]; then
  GOTMPDIR="$(dockpipe scope --package dorkpipe build go-tmp --workdir "$ROOT")"
  export GOTMPDIR
fi
mkdir -p "$GOCACHE" "$GOTMPDIR"

if [[ $# -eq 0 && -n "${DOCKPIPE_ARGS:-}" ]]; then
  # DOCKPIPE_ARGS is shell-joined by DockPipe for host-step CLI passthrough.
  # shellcheck disable=SC2086
  eval "set -- ${DOCKPIPE_ARGS}"
fi

if [[ -z "${DOCKPIPE_ARGS_JSON:-}" && $# -eq 0 ]]; then
  cat >&2 <<'EOF'
usage:
  dockpipe --package dorkpipe --workflow orchestrator -- --prompt "summarize this repo"
  dockpipe --package dorkpipe --workflow orchestrator -- --provider claude --prompt "review the package boundary"
  dockpipe --package dorkpipe --workflow orchestrator -- --provider ollama --model llama3.2 --prompt "quick local answer"
EOF
  exit 2
fi

dorkpipe_script_exec_cli "$SCRIPT_DIR" provider-pool prompt --workdir "$ROOT" "$@"
