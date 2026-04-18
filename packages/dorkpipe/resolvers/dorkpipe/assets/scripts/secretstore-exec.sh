#!/usr/bin/env bash
# Secret-store wrapper (bundled core): dotenv-style file → run SECRETSTORE_COMMAND.
# For 1Password (op), use workflow secretstore-onepassword (maintainer staging) and secretstore-op-exec.sh.
set -euo pipefail

eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"
ROOT="$(dockpipe_sdk workdir)"
dockpipe_sdk cd-workdir

PROVIDER="${SECRETSTORE_PROVIDER:-dotenv}"
case "$PROVIDER" in
  dotenv|file)
    ENV_FILE="${SECRETSTORE_ENV_FILE:-.env.secretstore}"
    [[ -f "$ENV_FILE" ]] || dockpipe_sdk die "missing $ENV_FILE — copy src/core/workflows/secretstore/.env.secretstore.example (see README)"
    CMD="${SECRETSTORE_COMMAND:-}"
    [[ -n "$CMD" ]] || dockpipe_sdk die "set SECRETSTORE_COMMAND (shell command to run with loaded env, e.g. dockpipe --workflow mywf --workdir . --)"
    set -a
    # shellcheck disable=SC1090
    . "$ENV_FILE"
    set +a
    exec bash -c "$CMD"
    ;;
  *)
    dockpipe_sdk die "unsupported SECRETSTORE_PROVIDER=$PROVIDER — bundled core supports dotenv|file only. For op (1Password), use --workflow secretstore-onepassword in this repo."
    ;;
esac
