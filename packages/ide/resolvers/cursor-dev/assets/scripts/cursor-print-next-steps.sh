#!/usr/bin/env bash
# Host-only follow-up (skip_container step). Not a Cursor product integration.
# For a long-lived container + docker wait, use cursor-dev-session.sh (cursor-dev workflow default).
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/cursor-dev-common.sh"

if [[ -f "${SCRIPT_DIR}/cursor-prep.sh" ]]; then
  bash "${SCRIPT_DIR}/cursor-prep.sh"
fi

cursor_dev_set_workdir
printf '[dockpipe] AI agent + MCP quickstart: %s/AGENT-MCP.md\n' "$(cursor_dev_state_root)" >&2

if ! cursor_dev_docker_preflight; then
  exit 1
fi

CURSOR_DEV_WAIT="${CURSOR_DEV_WAIT:-0}"

cursor_dev_print_instructions

if try_launch_cursor; then
  if [[ "${CURSOR_DEV_WAIT}" == "1" ]] && [[ -n "${LAUNCH_PID:-}" ]]; then
    printf '[dockpipe] Waiting for launcher PID %s (Ctrl+C stops dockpipe).\n' "${LAUNCH_PID}" >&2
    wait "${LAUNCH_PID}" || true
  fi
else
  printf '\n[cursor-dev] Cursor CLI not found in PATH and no default install path matched.\n' >&2
  printf '  Install Cursor from https://cursor.com and/or add the “cursor” shell command from the app.\n' >&2
fi

cursor_dev_footer
