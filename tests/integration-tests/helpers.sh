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
