# Core dockpipe runner: spawn container → run command → run action (if any).
# Sourced by the CLI. Uses: DOCKPIPE_IMAGE, DOCKPIPE_CMD (array), DOCKPIPE_ACTION,
# DOCKPIPE_WORKDIR, DOCKPIPE_DATA_VOLUME, DOCKPIPE_DATA_DIR, DOCKPIPE_EXTRA_MOUNTS, DOCKPIPE_EXTRA_ENV, DOCKPIPE_BUILD, DOCKPIPE_DETACH.

set -euo pipefail

# Default work directory inside the container (mount point for host cwd or repo).
export DOCKPIPE_CONTAINER_WORKDIR="${DOCKPIPE_CONTAINER_WORKDIR:-/work}"

# Run docker with the configured image, command, and optional action.
# Expects: DOCKPIPE_IMAGE, optional DOCKPIPE_ACTION path. Image must already exist or be built by CLI.
dockpipe_run() {
  local img="${DOCKPIPE_IMAGE:?DOCKPIPE_IMAGE is required}"
  local workdir_host="${DOCKPIPE_WORKDIR:-$(pwd)}"
  local action_path="${DOCKPIPE_ACTION:-}"
  local extra_mounts="${DOCKPIPE_EXTRA_MOUNTS:-}"
  local extra_env="${DOCKPIPE_EXTRA_ENV:-}"

  local -a run_args=(
    --rm
    --init
    -u "$(id -u):$(id -g)"
    -v "${workdir_host}:${DOCKPIPE_CONTAINER_WORKDIR}"
    -w "${DOCKPIPE_CONTAINER_WORKDIR}"
    -e "DOCKPIPE_CONTAINER_WORKDIR=${DOCKPIPE_CONTAINER_WORKDIR}"
  )

  # Mount and set action script if provided
  if [[ -n "${action_path}" ]] && [[ -f "${action_path}" ]]; then
    local action_name
    action_name="$(basename "${action_path}" .sh)"
    run_args+=(
      -v "$(realpath "${action_path}"):/dockpipe-action.sh:ro"
      -e "DOCKPIPE_ACTION=/dockpipe-action.sh"
    )
  fi

  # Data volume: persistent state (repos, tool config, first-time login). Either a bind mount (--data-dir) or a named volume (--data-vol, default dockpipe-data).
  if [[ -n "${DOCKPIPE_DATA_DIR:-}" ]]; then
    mkdir -p "${DOCKPIPE_DATA_DIR}"
    run_args+=(-v "${DOCKPIPE_DATA_DIR}:/dockpipe-data")
    run_args+=(
      -e "DOCKPIPE_DATA=/dockpipe-data"
      -e "HOME=/dockpipe-data"
    )
  elif [[ -n "${DOCKPIPE_DATA_VOLUME:-}" ]]; then
    run_args+=(-v "${DOCKPIPE_DATA_VOLUME}:/dockpipe-data")
    run_args+=(
      -e "DOCKPIPE_DATA=/dockpipe-data"
      -e "HOME=/dockpipe-data"
    )
  fi

  # Extra mounts: "host_path:container_path[:ro]" space-separated
  local m
  for m in ${extra_mounts}; do
    [[ -z "$m" ]] && continue
    run_args+=(-v "$m")
  done

  # Extra env: "KEY=VAL" space-separated
  local e
  for e in ${extra_env}; do
    [[ -z "$e" ]] && continue
    run_args+=(-e "$e")
  done

  # Attach vs detach: -d runs in background (container stays up until command exits)
  if [[ -n "${DOCKPIPE_DETACH:-}" ]]; then
    run_args+=(-d)
  else
    if [[ -t 0 ]]; then
      run_args+=(-it)
    else
      run_args+=(-i)
    fi
  fi

  # Run: image + command as arguments (exec form)
  exec docker run "${run_args[@]}" "${img}" "$@"
}
