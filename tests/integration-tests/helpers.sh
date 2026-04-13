#!/usr/bin/env bash

integration_repo_root() {
  cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd
}

require_agent_dev_template() {
  local repo_root claude_dockerfile
  repo_root="${1:-$(integration_repo_root)}"
  claude_dockerfile="$repo_root/.staging/packages/agent/resolvers/claude/assets/images/claude/Dockerfile"
  if [[ -f "$claude_dockerfile" ]]; then
    return 0
  fi
  echo "SKIP: optional agent-dev/claude template assets are not present in this checkout"
  exit 0
}

require_cursor_dev_script() {
  local repo_root cursor_script
  repo_root="${1:-$(integration_repo_root)}"
  cursor_script="$repo_root/packages/ide/resolvers/cursor-dev/assets/scripts/cursor-dev-session.sh"
  if [[ -f "$cursor_script" ]]; then
    REPLY="$cursor_script"
    return 0
  fi
  echo "FAIL: expected tracked cursor-dev package script at $cursor_script" >&2
  exit 1
}
