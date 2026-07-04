#!/usr/bin/env bash
set -euo pipefail

ROOT="${DOCKPIPE_SOURCE_ROOT:-$(pwd)}"
ARTIFACTS_DIR="${RELEASE_ARTIFACTS_DIR:-release/artifacts}"
SKIP_UPLOAD="${PACKAGE_RELEASE_SKIP_UPLOAD:-0}"
DRY_RUN="${R2_PUBLISH_DRY_RUN:-0}"

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

content_type_for() {
  case "$1" in
    *.json) printf '%s\n' 'application/json' ;;
    *.sha256) printf '%s\n' 'text/plain; charset=utf-8' ;;
    *.tar.gz) printf '%s\n' 'application/gzip' ;;
    *.zip) printf '%s\n' 'application/zip' ;;
    *.deb) printf '%s\n' 'application/vnd.debian.binary-package' ;;
    *.rpm) printf '%s\n' 'application/x-rpm' ;;
    *.msi) printf '%s\n' 'application/x-msi' ;;
    *.pkg.tar.zst) printf '%s\n' 'application/zstd' ;;
    *.apk) printf '%s\n' 'application/octet-stream' ;;
    *) printf '%s\n' 'application/octet-stream' ;;
  esac
}

normalize_prefix() {
  local prefix="${1:-}"
  prefix="${prefix#/}"
  prefix="${prefix%/}"
  printf '%s\n' "$prefix"
}

if [[ "$SKIP_UPLOAD" == "1" || "$SKIP_UPLOAD" == "true" ]]; then
  printf '[dockpipe] package-store-publish: PACKAGE_RELEASE_SKIP_UPLOAD=%s, skipping upload step\n' "$SKIP_UPLOAD" >&2
  exit 0
fi

if [[ ! -d "$ROOT/$ARTIFACTS_DIR" ]]; then
  printf '[dockpipe] package-store-publish: missing artifacts dir %s/%s\n' "$ROOT" "$ARTIFACTS_DIR" >&2
  exit 1
fi

PREFIX="$(normalize_prefix "${R2_PREFIX:-}")"
shopt -s nullglob
files=("$ROOT/$ARTIFACTS_DIR"/*)
shopt -u nullglob

if [[ ${#files[@]} -eq 0 ]]; then
  printf '[dockpipe] package-store-publish: no files under %s/%s\n' "$ROOT" "$ARTIFACTS_DIR" >&2
  exit 1
fi

for file in "${files[@]}"; do
  [[ -f "$file" ]] || continue
  name="$(basename "$file")"
  key="$name"
  if [[ -n "$PREFIX" ]]; then
    key="$PREFIX/$name"
  fi
  ct="$(content_type_for "$name")"
  args=(release upload "$file" --key "$key" --content-type "$ct")
  if [[ "$DRY_RUN" == "1" || "$DRY_RUN" == "true" ]]; then
    args+=(--dry-run)
  fi
  dockpipe_cli "${args[@]}"
done

printf '[dockpipe] package-store-publish: upload step complete for %s file(s)\n' "${#files[@]}" >&2
