#!/usr/bin/env bash
# Reusable Docker named-volume helpers for APT or any "cache across relaunches" path.
# Source:  source "$(git rev-parse --show-toplevel)/templates/core/assets/scripts/docker-cache-volumes.sh"
# Or run:  bash scripts/docker-cache-volumes.sh ensure -- "vol:/path" "vol2:/path2"
# (YAML paths like scripts/… resolve via paths.go — see docs/architecture.md.)
set -euo pipefail

docker_cache_volume_ensure() {
  local name="$1"
  docker volume inspect "$name" >/dev/null 2>&1 || docker volume create "$name" >/dev/null
}

# Appends docker -v flags to a named bash array (pass array name as first arg).
# Remaining args: "volume_name:/container/path" pairs (one word each).
docker_cache_volume_append_mounts() {
  local -n _out="$1"
  shift
  local pair vol path
  for pair in "$@"; do
    [[ -z "${pair}" ]] && continue
    vol="${pair%%:*}"
    path="${pair#*:}"
    if [[ "$vol" == "$pair" ]] || [[ -z "$path" ]]; then
      echo "docker-cache-volumes: bad vol:path pair: ${pair}" >&2
      return 1
    fi
    docker_cache_volume_ensure "$vol"
    _out+=(-v "${vol}:${path}")
  done
}

# Space-separated pairs in DOCKER_CACHE_VOLUMES (e.g. "dockpipe-apt-cache:/var/cache/apt dockpipe-apt-lists:/var/lib/apt/lists")
docker_cache_volume_append_from_env() {
  local -n _out="$1"
  local pair vol path
  for pair in ${DOCKER_CACHE_VOLUMES:-}; do
    [[ -z "${pair}" ]] && continue
    vol="${pair%%:*}"
    path="${pair#*:}"
    if [[ "$vol" == "$pair" ]] || [[ -z "$path" ]]; then
      echo "docker-cache-volumes: bad entry in DOCKER_CACHE_VOLUMES: ${pair}" >&2
      return 1
    fi
    docker_cache_volume_ensure "$vol"
    _out+=(-v "${vol}:${path}")
  done
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  case "${1:-}" in
  ensure)
    shift
    for pair in "$@"; do
      vol="${pair%%:*}"
      [[ "$vol" != "$pair" ]] || { echo "usage: $0 ensure vol:/path [vol2:/path2 ...]" >&2; exit 1; }
      docker_cache_volume_ensure "$vol"
      echo "ok ${vol}"
    done
    ;;
  *)
    echo "usage: source $0   # for docker_cache_volume_* functions" >&2
    echo "       $0 ensure vol:/path [vol2:/path2 ...]" >&2
    exit 1
    ;;
  esac
fi
