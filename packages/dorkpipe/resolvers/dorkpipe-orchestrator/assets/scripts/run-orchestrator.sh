#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../../dorkpipe/assets/scripts/lib/dorkpipe-cli.sh"
ROOT="$(pwd)"
WORKFLOW_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

BIN="$(dorkpipe_script_resolve_bin "$(dorkpipe_script_repo_root "$SCRIPT_DIR")")"
SPEC="${DORKPIPE_SPEC:-$WORKFLOW_ROOT/spec.example.yaml}"

if [[ ! -x "$BIN" ]]; then
  echo "dorkpipe: dorkpipe CLI not available from packaged assets, repo-local builds, or PATH" >&2
  echo "dorkpipe: consumer path expects compiled package assets or an installed dorkpipe binary; maintainer fallback: dockpipe package build source --workdir . --only dorkpipe" >&2
  exit 1
fi

exec "$BIN" run -f "$SPEC" --workdir "$ROOT"
