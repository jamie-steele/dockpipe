#!/usr/bin/env bash
# Back-compat wrapper: prefer dockpipe-sdk.sh for new scripts.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/dockpipe-sdk.sh"
