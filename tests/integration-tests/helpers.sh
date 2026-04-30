#!/usr/bin/env bash

integration_use_repo_checkout() {
  local repo_root
  repo_root="${1:-$(integration_repo_root)}"
  export DOCKPIPE_REPO_ROOT="$repo_root"
}

integration_repo_root() {
  cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd
}

require_agent_dev_template() {
  local repo_root claude_dockerfile
  repo_root="${1:-$(integration_repo_root)}"
  claude_dockerfile="$(find "$repo_root" -path '*/assets/images/claude/Dockerfile' -print -quit 2>/dev/null || true)"
  if [[ -n "$claude_dockerfile" ]] && [[ -f "$claude_dockerfile" ]]; then
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

# These integration tests exercise the repo checkout binary and assets, not the
# installed bundled cache. Export the checkout root when the helper is sourced
# so individual test files behave the same way as the full harness.
integration_use_repo_checkout
