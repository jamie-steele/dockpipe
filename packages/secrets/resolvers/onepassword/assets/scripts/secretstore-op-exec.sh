#!/usr/bin/env bash
# 1Password CLI: op run --env-file for the onepassword package workflow.
set -euo pipefail

eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"
ROOT="$(dockpipe_sdk workdir)"
dockpipe_sdk cd-workdir

PROVIDER="${SECRETSTORE_PROVIDER:-op}"
case "$PROVIDER" in
  op)
    command -v op >/dev/null 2>&1 || dockpipe_sdk die "install 1Password CLI (https://developer.1password.com/docs/cli/)"
    OP_ENV_FILE="${OP_ENV_FILE:-.env.op.template}"
    [[ -f "$OP_ENV_FILE" ]] || dockpipe_sdk die "missing $OP_ENV_FILE — copy packages/secrets/resolvers/onepassword/.env.op.template.example to .env.op.template and add op:// lines"
    CMD="${SECRETSTORE_COMMAND:-}"
    [[ -n "$CMD" ]] || dockpipe_sdk die "set SECRETSTORE_COMMAND (shell command to run with injected env, e.g. dockpipe --workflow mywf --workdir . --)"
    exec op run --env-file="$OP_ENV_FILE" -- bash -c "$CMD"
    ;;
  *)
    dockpipe_sdk die "unsupported SECRETSTORE_PROVIDER=$PROVIDER — this script supports op only"
    ;;
esac
