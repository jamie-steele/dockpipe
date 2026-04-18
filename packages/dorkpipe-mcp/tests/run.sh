#!/usr/bin/env bash
# Self-contained Go tests for dorkpipe-mcp (`dorkpipe.mcp`).
# From repo root: bash packages/dorkpipe-mcp/tests/run.sh
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
MCP_ROOT="$ROOT/packages/dorkpipe-mcp"
cd "$MCP_ROOT"
go test ./...
