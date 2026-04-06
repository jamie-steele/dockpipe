#!/usr/bin/env bash
# Self-contained Go tests for dorkpipe-mcp (`dorkpipe.mcp`).
# From repo root: bash packages/dorkpipe-mcp/tests/run.sh
set -euo pipefail
MCP_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$MCP_ROOT"
go test ./...
