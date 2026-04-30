#!/usr/bin/env bash
set -euo pipefail

eval "$(dockpipe sdk)"
dockpipe_sdk init-script

CONTAINER_CLI="${GHCR_CONTAINER_CLI:-docker}"
SOURCE_IMAGE="${GHCR_IMAGE_SOURCE:-}"
TARGET_IMAGE="${GHCR_IMAGE_TARGET:-}"
REGISTRY="${GHCR_REGISTRY:-ghcr.io}"
LOGIN_SKIP="${GHCR_LOGIN_SKIP:-0}"
DRY_RUN="${GHCR_PUSH_DRY_RUN:-0}"
USERNAME="${GHCR_USERNAME:-${GITHUB_ACTOR:-}}"
TOKEN="${GHCR_TOKEN:-${GITHUB_TOKEN:-}}"

command -v "$CONTAINER_CLI" >/dev/null 2>&1 || dockpipe_sdk die "container cli not found: $CONTAINER_CLI"
[[ -n "$SOURCE_IMAGE" ]] || dockpipe_sdk die "set GHCR_IMAGE_SOURCE"
[[ -n "$TARGET_IMAGE" ]] || dockpipe_sdk die "set GHCR_IMAGE_TARGET"

normalize_target_image() {
  local image="$1"
  case "$image" in
    "${REGISTRY}/"*) printf '%s\n' "$image" ;;
    *) printf '%s/%s\n' "$REGISTRY" "$image" ;;
  esac
}

image_has_explicit_tag() {
  local image="$1"
  local last_segment="${image##*/}"
  [[ "$last_segment" == *:* || "$image" == *@* ]]
}

image_repository_ref() {
  local image="$1"
  if [[ "$image" == *@* ]]; then
    printf '%s\n' "${image%@*}"
    return 0
  fi
  local last_segment="${image##*/}"
  if [[ "$last_segment" == *:* ]]; then
    printf '%s\n' "${image%:*}"
    return 0
  fi
  printf '%s\n' "$image"
}

trim_value() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s\n' "$value"
}

collect_tags() {
  local raw="${GHCR_TAGS:-}"
  local normalized
  normalized="$(printf '%s' "$raw" | tr ',\n' '  ')"
  local tag
  for tag in $normalized; do
    tag="$(trim_value "$tag")"
    [[ -n "$tag" ]] && printf '%s\n' "$tag"
  done
}

TARGET_IMAGE="$(normalize_target_image "$TARGET_IMAGE")"
TARGET_REPOSITORY="$(image_repository_ref "$TARGET_IMAGE")"

declare -a TARGET_REFS=()

if [[ -n "${GHCR_TAGS:-}" ]]; then
  while IFS= read -r tag; do
    [[ -n "$tag" ]] || continue
    TARGET_REFS+=("${TARGET_REPOSITORY}:$tag")
  done < <(collect_tags)
  [[ ${#TARGET_REFS[@]} -gt 0 ]] || dockpipe_sdk die "GHCR_TAGS was provided but no tags were parsed"
elif image_has_explicit_tag "$TARGET_IMAGE"; then
  TARGET_REFS+=("$TARGET_IMAGE")
else
  TARGET_REFS+=("${TARGET_IMAGE}:latest")
fi

"$CONTAINER_CLI" image inspect "$SOURCE_IMAGE" >/dev/null 2>&1 || dockpipe_sdk die "source image not found locally: $SOURCE_IMAGE"

if [[ "$LOGIN_SKIP" != "1" ]]; then
  [[ -n "$USERNAME" ]] || dockpipe_sdk die "set GHCR_USERNAME or GITHUB_ACTOR (or set GHCR_LOGIN_SKIP=1)"
  [[ -n "$TOKEN" ]] || dockpipe_sdk die "set GHCR_TOKEN or GITHUB_TOKEN (or set GHCR_LOGIN_SKIP=1)"
fi

if [[ "$DRY_RUN" == "1" ]]; then
  echo "${WF_NS}: DRY RUN — would push from ${SOURCE_IMAGE}"
  if [[ "$LOGIN_SKIP" == "1" ]]; then
    echo "${WF_NS}: would reuse existing ${CONTAINER_CLI} auth for ${REGISTRY}"
  else
    echo "${WF_NS}: would log into ${REGISTRY} as ${USERNAME}"
  fi
  for target_ref in "${TARGET_REFS[@]}"; do
    echo "${WF_NS}: would tag ${SOURCE_IMAGE} -> ${target_ref}"
    echo "${WF_NS}: would push ${target_ref}"
  done
  exit 0
fi

if [[ "$LOGIN_SKIP" != "1" ]]; then
  login_approval="$(
    dockpipe_sdk prompt confirm \
      --id ghcr_registry_login \
      --title "Allow Registry Login?" \
      --message "DockPipe will log into ${REGISTRY} with ${CONTAINER_CLI}, which may update local registry credentials on this machine. Continue?" \
      --default no \
      --intent credential-use \
      --automation-group registry-login \
      --allow-auto-approve \
      --auto-approve-value yes
  )" || exit 1
  if [[ "$login_approval" != "yes" ]]; then
    echo "${WF_NS}: cancelled before registry login" >&2
    exit 0
  fi
  printf '%s\n' "$TOKEN" | "$CONTAINER_CLI" login "$REGISTRY" -u "$USERNAME" --password-stdin
fi

for target_ref in "${TARGET_REFS[@]}"; do
  "$CONTAINER_CLI" tag "$SOURCE_IMAGE" "$target_ref"
  "$CONTAINER_CLI" push "$target_ref"
  echo "${WF_NS}: pushed ${target_ref}"
done
