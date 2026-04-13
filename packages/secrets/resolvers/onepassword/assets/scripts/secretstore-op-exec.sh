#!/usr/bin/env bash
# 1Password CLI: op run --env-file for the onepassword package workflow.
set -euo pipefail

ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
cd "$ROOT"

die() { echo "secretstore-op: $*" >&2; exit 1; }

PROVIDER="${SECRETSTORE_PROVIDER:-op}"
case "$PROVIDER" in
  op)
    command -v op >/dev/null 2>&1 || die "install 1Password CLI (https://developer.1password.com/docs/cli/)"
    OP_ENV_FILE="${OP_ENV_FILE:-.env.op.template}"
    [[ -f "$OP_ENV_FILE" ]] || die "missing $OP_ENV_FILE — copy packages/secrets/resolvers/onepassword/.env.op.template.example to .env.op.template and add op:// lines"
    CMD="${SECRETSTORE_COMMAND:-}"
    [[ -n "$CMD" ]] || die "set SECRETSTORE_COMMAND (shell command to run with injected env, e.g. dockpipe --workflow mywf --workdir . --)"
    exec op run --env-file="$OP_ENV_FILE" -- bash -c "$CMD"
    ;;
  *)
    die "unsupported SECRETSTORE_PROVIDER=$PROVIDER — this script supports op only"
    ;;
esac
