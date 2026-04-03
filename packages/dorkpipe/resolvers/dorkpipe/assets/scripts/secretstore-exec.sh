#!/usr/bin/env bash
# Secret-store wrapper (bundled core): dotenv-style file → run SECRETSTORE_COMMAND.
# For 1Password (op), use workflow secretstore-onepassword (maintainer staging) and secretstore-op-exec.sh.
set -euo pipefail

ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
cd "$ROOT"

die() { echo "secretstore: $*" >&2; exit 1; }

PROVIDER="${SECRETSTORE_PROVIDER:-dotenv}"
case "$PROVIDER" in
  dotenv|file)
    ENV_FILE="${SECRETSTORE_ENV_FILE:-.env.secretstore}"
    [[ -f "$ENV_FILE" ]] || die "missing $ENV_FILE — copy src/core/workflows/secretstore/.env.secretstore.example (see README)"
    CMD="${SECRETSTORE_COMMAND:-}"
    [[ -n "$CMD" ]] || die "set SECRETSTORE_COMMAND (shell command to run with loaded env, e.g. ./src/bin/dockpipe --workflow mywf --workdir . --)"
    set -a
    # shellcheck disable=SC1090
    . "$ENV_FILE"
    set +a
    exec bash -c "$CMD"
    ;;
  *)
    die "unsupported SECRETSTORE_PROVIDER=$PROVIDER — bundled core supports dotenv|file only. For op (1Password), use --workflow secretstore-onepassword in this repo."
    ;;
esac
