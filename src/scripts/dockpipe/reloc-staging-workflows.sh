#!/usr/bin/env bash
# One-time migration: nest flat .staging/workflows dirs under dockpipe/packages/*/resolvers (legacy layout).
# This repo gitignores .staging/ — use plain mv (not git mv). Run from repo root:
#   bash src/scripts/dockpipe/reloc-staging-workflows.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT/.staging/workflows"

if [[ ! -d dockpipe-agents ]] && [[ ! -d dockpipe-ide ]] && [[ ! -d dockpipe.secrets.1password ]] && [[ ! -d dockpipe.storage.cloudflare.r2 ]]; then
  echo "Nothing to relocate (flat dirs already moved)."
  exit 0
fi

mkdir -p dockpipe/packages/agents/resolvers dockpipe/packages/ides/resolvers dockpipe/packages/secrets/resolvers dockpipe/storage/cloudflare/r2

if [[ -d dockpipe-agents ]]; then
  for d in codex claude ollama; do
    [[ -d "dockpipe-agents/$d" ]] || continue
    mv "dockpipe-agents/$d" "dockpipe/packages/agents/resolvers/$d"
  done
  rmdir dockpipe-agents 2>/dev/null || true
fi

if [[ -d dockpipe-ide ]]; then
  for d in vscode cursor-dev code-server; do
    [[ -d "dockpipe-ide/$d" ]] || continue
    mv "dockpipe-ide/$d" "dockpipe/packages/ides/resolvers/$d"
  done
  rmdir dockpipe-ide 2>/dev/null || true
fi

if [[ -d dockpipe.secrets.1password ]]; then
  mv dockpipe.secrets.1password dockpipe/packages/secrets/resolvers/onepassword
fi

if [[ -d dockpipe.storage.cloudflare.r2 ]]; then
  [[ -d dockpipe.storage.cloudflare.r2/dockpipe.cloudflare.r2publish ]] \
    && mv dockpipe.storage.cloudflare.r2/dockpipe.cloudflare.r2publish dockpipe/storage/cloudflare/r2/dockpipe.cloudflare.r2publish
  [[ -d dockpipe.storage.cloudflare.r2/secretstore-r2-publish-test ]] \
    && mv dockpipe.storage.cloudflare.r2/secretstore-r2-publish-test dockpipe/storage/cloudflare/r2/secretstore-r2-publish-test
  rmdir dockpipe.storage.cloudflare.r2 2>/dev/null || true
fi

echo "Done. From repo root: go test ./... && bash tests/unit-tests/test_repo_layout.sh"
