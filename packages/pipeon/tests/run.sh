#!/usr/bin/env bash
# Self-contained tests for the first-party pipeon package.
# From repo root: bash packages/pipeon/tests/run.sh
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bash "$DIR/test_pipeon.sh"
bash "$DIR/test_repo_tools.sh"
