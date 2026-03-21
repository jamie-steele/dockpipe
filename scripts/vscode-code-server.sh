#!/usr/bin/env bash
# Host-run code-server (OSS) with published port. Dockpipe's container runner does not map host
# ports; this script invokes docker directly so the browser can reach code-server.
set -euo pipefail

WORKDIR="${DOCKPIPE_WORKDIR:-$PWD}"
WORKDIR="$(cd "$WORKDIR" && pwd)"

IMAGE="${CODE_SERVER_IMAGE:-codercom/code-server:latest}"
PORT="${CODE_SERVER_PORT:-8080}"
NAME="${CODE_SERVER_CONTAINER_NAME:-dockpipe-code-server-${PORT}}"

if [[ -z "${CODE_SERVER_PASSWORD:-}" ]]; then
  if command -v openssl >/dev/null 2>&1; then
    CODE_SERVER_PASSWORD="$(openssl rand -hex 8)"
  else
    CODE_SERVER_PASSWORD="dockpipe-$(date +%s)"
  fi
  printf '[dockpipe] CODE_SERVER_PASSWORD was unset; using a generated value (set CODE_SERVER_PASSWORD in vars or .env to fix).\n' >&2
fi

docker rm -f "$NAME" 2>/dev/null || true

if [[ "${DOCKPIPE_SKIP_PULL:-}" != "1" ]]; then
  printf '[dockpipe] Pulling image %s …\n' "$IMAGE" >&2
  docker pull "$IMAGE" >&2
fi

args=(
  run -d --rm --name "$NAME"
  -p "${PORT}:8080"
  -v "${WORKDIR}:/work"
  -w /work
  -e "PASSWORD=${CODE_SERVER_PASSWORD}"
)

# Match bind-mount ownership on Unix hosts (Docker Desktop on Windows often omits -u).
case "${OSTYPE:-}" in
  linux-gnu*|darwin*)
    args+=( -u "$(id -u):$(id -g)" )
    ;;
esac

args+=( "$IMAGE" --bind-addr "0.0.0.0:8080" /work )

docker "${args[@]}" >/dev/null

printf '\n[dockpipe] code-server (OSS) is running.\n' >&2
printf '  URL:      http://127.0.0.1:%s/\n' "$PORT" >&2
printf '  Password: %s\n' "$CODE_SERVER_PASSWORD" >&2
printf '  Stop:     docker stop %s\n' "$NAME" >&2
printf '\nThird-party image (%s); not Microsoft VS Code. See template README.\n' "$IMAGE" >&2
