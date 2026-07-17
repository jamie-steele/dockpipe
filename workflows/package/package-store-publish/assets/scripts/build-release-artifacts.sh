#!/usr/bin/env bash
set -euo pipefail

ROOT="${DOCKPIPE_SOURCE_ROOT:-$(pwd)}"
ARTIFACTS_DIR="${RELEASE_ARTIFACTS_DIR:-release/artifacts}"
PREPARE_ASSETS="${PACKAGE_RELEASE_PREPARE_EMBEDDED_ASSETS:-1}"

is_windows_host() {
  case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*) return 0 ;;
    *) [[ "${OS:-}" == "Windows_NT" ]] ;;
  esac
}

dockpipe_bin_path() {
  if is_windows_host; then
    printf '%s\n' "$ROOT/src/bin/dockpipe.exe"
  else
    printf '%s\n' "$ROOT/src/bin/dockpipe"
  fi
}

dockpipe_cli() {
  local bin
  bin="$(dockpipe_bin_path)"
  if [[ -x "$bin" ]]; then
    "$bin" "$@"
    return
  fi
  (
    cd "$ROOT"
    go run ./src/cmd "$@"
  )
}

build_repo_dockpipe() {
  local version="$1"
  local out
  out="$(dockpipe_bin_path)"
  local -a args=(go build -trimpath -o "$out")
  if [[ -n "$version" ]]; then
    args+=(-ldflags "-s -w -X main.Version=${version}")
  fi
  args+=(./src/cmd)
  (
    cd "$ROOT"
    "${args[@]}"
  )
}

read_release_version() {
  if [[ -n "${RELEASE_VERSION:-}" ]]; then
    printf '%s\n' "$RELEASE_VERSION"
    return
  fi
  tr -d '\r' < "$ROOT/VERSION" | head -n 1
}

VERSION="$(read_release_version)"
mkdir -p "$ROOT/$ARTIFACTS_DIR"

if [[ "$PREPARE_ASSETS" == "1" || "$PREPARE_ASSETS" == "true" ]]; then
  (
    cd "$ROOT"
    ./release/packaging/prepare-embedded-dorkpipe-assets.sh prepare
  )
fi

build_repo_dockpipe "$VERSION"

dockpipe_cli build --workdir "$ROOT" --no-images
dockpipe_cli package build core --repo-root "$ROOT" --out "$ARTIFACTS_DIR" --version "$VERSION"
dockpipe_cli package build store --workdir "$ROOT" --out "$ARTIFACTS_DIR" --version "$VERSION"

printf '[dockpipe] package-store-publish: artifacts ready in %s/%s\n' "$ROOT" "$ARTIFACTS_DIR" >&2
