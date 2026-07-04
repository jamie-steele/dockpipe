#!/usr/bin/env bash
# Bundled templates/scripts/images are embedded in the Go binary and materialize to the user cache.
# Repo root resolution is covered by Go tests (src/lib/infrastructure).
set -euo pipefail

echo "test_repo_root OK (see TestRepoRootMaterializesBundledTemplates in infrastructure tests)"
exit 0
