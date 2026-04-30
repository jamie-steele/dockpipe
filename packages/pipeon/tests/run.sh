#!/usr/bin/env bash
# Self-contained tests for the first-party pipeon package.
# From repo root: dockpipe package test --only pipeon
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
DIR="$ROOT/packages/pipeon/tests"
bash "$DIR/test_pipeon.sh"
bash "$DIR/test_repo_tools.sh"
bash "$DIR/test_sdk_prompt.sh"
