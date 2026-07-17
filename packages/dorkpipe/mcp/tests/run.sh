#!/usr/bin/env bash
# Self-contained Go tests for DorkPipe MCP (`dorkpipe.mcp`).
# From repo root: dockpipe package test --only dorkpipe.mcp
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
MCP_ROOT="$ROOT/packages/dorkpipe/mcp"
cd "$MCP_ROOT"
unset DOCKPIPE_WORKDIR
unset DOCKPIPE_REPO_ROOT
go test ./...
