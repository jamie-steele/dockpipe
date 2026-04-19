#!/usr/bin/env bash
set -euo pipefail

MCPD_BIN="${PIPEON_DEV_STACK_MCPD_BIN:-/repo/packages/dorkpipe/bin/mcpd}"
MCPD_LISTEN="${PIPEON_DEV_STACK_MCPD_LISTEN:-127.0.0.1:8765}"
PROXY_PORT="${PIPEON_MCP_PROXY_PORT:-8766}"
UPSTREAM_URL="${PIPEON_MCP_UPSTREAM_URL:-http://127.0.0.1:8765}"
PROXY_SCRIPT="${PIPEON_MCP_PROXY_SCRIPT:-/repo/packages/pipeon/resolvers/pipeon-dev-stack/assets/scripts/mcp-proxy.js}"

if [[ ! -x "$MCPD_BIN" ]]; then
  echo "dorkpipe-stack: mcpd binary not executable at $MCPD_BIN" >&2
  exit 1
fi

if [[ ! -f "$PROXY_SCRIPT" ]]; then
  echo "dorkpipe-stack: MCP proxy script not found at $PROXY_SCRIPT" >&2
  exit 1
fi

cleanup() {
  if [[ -n "${MCPD_PID:-}" ]] && kill -0 "$MCPD_PID" 2>/dev/null; then
    kill "$MCPD_PID" >/dev/null 2>&1 || true
    wait "$MCPD_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

"$MCPD_BIN" -http "$MCPD_LISTEN" -insecure-loopback &
MCPD_PID=$!

for _ in $(seq 1 40); do
  if kill -0 "$MCPD_PID" 2>/dev/null; then
    if command -v curl >/dev/null 2>&1; then
      code="$(
        curl -sS -o /dev/null -w '%{http_code}' "$UPSTREAM_URL" 2>/dev/null || true
      )"
      case "$code" in
        200|204|400|401|405)
          break
          ;;
      esac
    else
      sleep 0.25
      break
    fi
  else
    echo "dorkpipe-stack: mcpd exited during startup" >&2
    wait "$MCPD_PID"
    exit 1
  fi
  sleep 0.25
done

export PIPEON_MCP_PROXY_PORT="$PROXY_PORT"
export PIPEON_MCP_UPSTREAM_URL="$UPSTREAM_URL"
exec node "$PROXY_SCRIPT"
