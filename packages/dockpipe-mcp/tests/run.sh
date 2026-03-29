#!/usr/bin/env bash
# Self-contained Go tests for dockpipe-mcp (`dockpipe.mcp`).
# From repo root: bash packages/dockpipe-mcp/tests/run.sh
set -euo pipefail
MCP_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$MCP_ROOT"
go test ./...
