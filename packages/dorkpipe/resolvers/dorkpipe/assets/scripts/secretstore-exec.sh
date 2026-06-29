#!/usr/bin/env bash
# Secret-store wrapper (bundled core): dotenv-style file → run SECRETSTORE_COMMAND.
# For 1Password (op), use workflow secretstore-onepassword (maintainer staging) and secretstore-op-exec.sh.
set -euo pipefail

ROOT="$(dockpipe get workdir)"
cd "$ROOT"

die() { echo "dorkpipe: $*" >&2; exit 1; }

resolve_host_bash_bin() {
  local candidate
  for candidate in \
    "${DOCKPIPE_HOST_BASH_BIN:-}" \
    "${BASH:-}" \
    "$(command -v bash 2>/dev/null || true)"
  do
    if [[ -n "$candidate" && -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

PROVIDER="${SECRETSTORE_PROVIDER:-dotenv}"
case "$PROVIDER" in
  dotenv|file)
    ENV_FILE="${SECRETSTORE_ENV_FILE:-.env.secretstore}"
    [[ -f "$ENV_FILE" ]] || die "missing $ENV_FILE — copy src/core/workflows/secretstore/.env.secretstore.example (see README)"
    CMD="${SECRETSTORE_COMMAND:-}"
    [[ -n "$CMD" ]] || die "set SECRETSTORE_COMMAND (shell command to run with loaded env, e.g. dockpipe --workflow mywf --workdir . --)"
    set -a
    # shellcheck disable=SC1090
    . "$ENV_FILE"
    set +a
    HOST_BASH_BIN="$(resolve_host_bash_bin)" || die "bash executable not found for SECRETSTORE_COMMAND"
    exec "$HOST_BASH_BIN" -c "$CMD"
    ;;
  *)
    die "unsupported SECRETSTORE_PROVIDER=$PROVIDER — bundled core supports dotenv|file only. For op (1Password), use --workflow secretstore-onepassword in this repo."
    ;;
esac
