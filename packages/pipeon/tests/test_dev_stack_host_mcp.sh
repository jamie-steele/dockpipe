#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
launch="$ROOT/packages/pipeon/resolvers/pipeon-dev-stack/assets/scripts/launch.sh"
common="$ROOT/packages/pipeon/resolvers/pipeon-dev-stack/assets/scripts/common.sh"
desktop="$ROOT/packages/pipeon/resolvers/pipeon-dev-stack/assets/scripts/desktop.sh"

if ! grep -q 'DOCKPIPE_MCP_ALLOWED_TOOLS= \\' "$launch"; then
  echo "pipeon-dev-stack host bridge must clear inherited DOCKPIPE_MCP_ALLOWED_TOOLS so new bridge tools are not denied by stale parent env" >&2
  exit 1
fi
if ! grep -q 'DOCKPIPE_MCP_IGNORE_ALLOWED_TOOLS=1 \\' "$launch"; then
  echo "pipeon-dev-stack host bridge must ignore inherited allowed-tool filters on Windows" >&2
  exit 1
fi
if ! grep -q 'Get-NetTCPConnection -LocalAddress 127.0.0.1 -LocalPort' "$launch"; then
  echo "pipeon-dev-stack host bridge cleanup must stop stale Windows port owners, not only pid-file pids" >&2
  exit 1
fi

for tool in \
  dorkpipe.provider_pool_catalog \
  dorkpipe.provider_pool_status \
  dorkpipe.provider_pool_chat \
  dorkpipe.host_codex_chat \
  dorkpipe.host_claude_chat \
  dorkpipe.host_claude_auth \
  dorkpipe.provider_auth_status \
  dorkpipe.provider_auth_repair
do
  if ! grep -q "\"$tool\"" "$launch"; then
    echo "missing host MCP bridge tool in pipeon-dev-stack reuse probe: $tool" >&2
    exit 1
  fi
done

if ! grep -q 'pipeon_stack_powershell_hidden()' "$common"; then
  echo "missing hidden PowerShell helper for non-interactive Pipeon host calls" >&2
  exit 1
fi

if ! grep -q -- '-WindowStyle Hidden' "$common"; then
  echo "hidden PowerShell helper does not set -WindowStyle Hidden on Windows" >&2
  exit 1
fi

if grep -q '"\$powershell_bin" -NoProfile -Command' "$desktop"; then
  echo "desktop launch still invokes non-interactive PowerShell without hidden helper" >&2
  exit 1
fi

if ! grep -q 'ollama_model_available()' "$launch"; then
  echo "missing cached Ollama model check before launch-time pull" >&2
  exit 1
fi

if ! grep -q 'Ollama model .* is already present; skipping pull' "$launch"; then
  echo "missing launch-time Ollama pull skip message" >&2
  exit 1
fi

echo "pipeon-dev-stack host MCP, hidden PowerShell, and model-cache checks ok"
