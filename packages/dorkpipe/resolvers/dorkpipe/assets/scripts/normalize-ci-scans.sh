#!/usr/bin/env bash
# Normalize gosec + govulncheck JSON into DorkPipe CI artifact state.
# for DorkPipe downstream reasoning.
# Thin wrapper around `dorkpipe ci normalize-scans`.
set -euo pipefail

if [[ -n "${DOCKPIPE_SCRIPT_DIR:-}" ]]; then
	SCRIPT_DIR="$DOCKPIPE_SCRIPT_DIR"
else
	SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
	SCRIPT_DIR="$SOURCE_DIR"
fi
# shellcheck source=/dev/null
source "$SCRIPT_DIR/lib/dorkpipe-cli.sh"
ROOT="${DOCKPIPE_WORKDIR:-}"
if [[ -z "$ROOT" ]]; then
	ROOT="$(dorkpipe_script_repo_root "$SCRIPT_DIR")"
fi
[[ -n "$ROOT" ]] || dorkpipe_script_die "DOCKPIPE_WORKDIR is required when repo root cannot be inferred"

dorkpipe_script_exec_cli "$SCRIPT_DIR" ci normalize-scans --workdir "$ROOT"
