#!/usr/bin/env bash
# Secret-store wrapper: dispatches on SECRETSTORE_PROVIDER (default op). Used by templates/secretstore.
set -euo pipefail

ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
cd "$ROOT"

die() { echo "secretstore: $*" >&2; exit 1; }

PROVIDER="${SECRETSTORE_PROVIDER:-op}"
case "$PROVIDER" in
  op)
    command -v op >/dev/null 2>&1 || die "install 1Password CLI (https://developer.1password.com/docs/cli/)"
    OP_ENV_FILE="${OP_ENV_FILE:-.env.op.template}"
    [[ -f "$OP_ENV_FILE" ]] || die "missing $OP_ENV_FILE — copy templates/secretstore/.env.op.template.example and add op:// lines"
    CMD="${SECRETSTORE_COMMAND:-}"
    [[ -n "$CMD" ]] || die "set SECRETSTORE_COMMAND (shell command to run with injected env, e.g. ./src/bin/dockpipe --workflow mywf --workdir . --)"
    exec op run --env-file="$OP_ENV_FILE" -- bash -c "$CMD"
    ;;
  *)
    die "unsupported SECRETSTORE_PROVIDER=$PROVIDER — extend src/scripts/dockpipe/secretstore-exec.sh (known: op)"
    ;;
esac
