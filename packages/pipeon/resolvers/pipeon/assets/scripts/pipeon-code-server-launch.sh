#!/usr/bin/env bash
# Launched by the Pipeon desktop shortcut. Starts workflow vscode (code-server + Pipeon image).
set -euo pipefail

cd "${PIPEON_WORKDIR:-$HOME}"
exec dockpipe --workflow vscode --workdir .
