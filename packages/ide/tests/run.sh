#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
exec node "$ROOT/packages/ide/tests/devcontainer-lifecycle.test.js"
