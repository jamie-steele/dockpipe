#!/usr/bin/env bash
# Back-compat wrapper: prefer dockpipe-sdk.sh for new scripts.
SOURCE_DIR="${BASH_SOURCE[0]}"
SOURCE_DIR="${SOURCE_DIR%/*}"
[[ "$SOURCE_DIR" == "${BASH_SOURCE[0]}" ]] && SOURCE_DIR="."
SCRIPT_DIR="$(cd "$SOURCE_DIR" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/dockpipe-sdk.sh"
