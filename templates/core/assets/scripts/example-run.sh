#!/usr/bin/env bash
# Example run script (runs on host before container). Use as a starting point; replace with your workflow.
# Config points to scripts/example-run.sh; override with --run or use dockpipe --workflow <name>.
set -euo pipefail

# Minimal: run a command in the example image with current dir at /work.
IMAGE="${IMAGE:-dockpipe-example}"
docker run --rm -v "$(pwd):/work" -w /work "${IMAGE}" "${@:-echo 'Edit scripts/example-run.sh or use dockpipe --workflow <name> -- ...'}"
