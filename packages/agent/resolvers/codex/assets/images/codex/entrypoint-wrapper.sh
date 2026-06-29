#!/usr/bin/env bash
set -euo pipefail

# Clear sudo-like markers some CLIs interpret as elevated mode.
unset SUDO_COMMAND SUDO_USER SUDO_UID SUDO_GID 2>/dev/null || true

# Some hosts still start the container as root even when the image sets USER node.
# Codex behaves better when it runs as node in the disposable resolver image.
if [[ -z "${DOCKPIPE_SKIP_DROP_TO_NODE:-}" ]] && [[ "$(id -u)" -eq 0 ]] && [[ -z "${DOCKPIPE_ENTRYPOINT_DROPPED:-}" ]] && id -u node &>/dev/null; then
  export DOCKPIPE_ENTRYPOINT_DROPPED=1
  if command -v runuser >/dev/null 2>&1; then
    exec runuser -u node -- /entrypoint.sh "$@"
  fi
  if command -v setpriv >/dev/null 2>&1; then
    exec setpriv --reuid="$(id -u node)" --regid="$(id -g node)" --init-groups -- /entrypoint.sh "$@"
  fi
  echo "[dockpipe] codex entrypoint: FATAL: need runuser or setpriv (util-linux) to drop root -> node" >&2
  exit 1
fi

# Claude/Codex-style CLIs treat IS_SANDBOX=1 as disposable-sandbox mode.
if [[ -z "${DOCKPIPE_NO_SANDBOX_ENV:-}" ]] && [[ -z "${IS_SANDBOX:-}" ]]; then
  export IS_SANDBOX=1
fi

exec /entrypoint.sh "$@"
