#!/usr/bin/env bash
# Refresh .staging/resolvers/ from templates (packaging mirror). Committed .staging/workflows/
# is curated separately — this script does not overwrite it.
#
# Core (runtimes, strategies, assets, bundles) stays under src/templates/core for release tarballs.
# See docs/package-model.md.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
mkdir -p .staging
rsync -a --delete "${ROOT}/src/templates/core/resolvers/" "${ROOT}/.staging/resolvers/"
echo "Refreshed ${ROOT}/.staging/resolvers (workflows under .staging/workflows/ are not touched)"
