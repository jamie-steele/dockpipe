#!/usr/bin/env bash
set -euo pipefail

dorkpipe_stack_state_dir() {
  if [[ -n "${DOCKPIPE_PACKAGE_STATE_DIR:-}" ]]; then
    printf '%s\n' "$DOCKPIPE_PACKAGE_STATE_DIR"
    return 0
  fi
  printf '%s/bin/.dockpipe/packages/dorkpipe-dev-stack\n' "$REPO_ROOT"
}

dorkpipe_stack_ensure_state_dir() {
  mkdir -p "$(dorkpipe_stack_state_dir)"
}

dorkpipe_stack_gpu_compose_file() {
  printf '%s/docker-compose.gpu.yml\n' "$(dorkpipe_stack_state_dir)"
}

dorkpipe_stack_gpu_mode_file() {
  printf '%s/gpu.mode\n' "$(dorkpipe_stack_state_dir)"
}

dorkpipe_stack_bootstrap_sdk() {
  local dockpipe_bin
  for dockpipe_bin in \
    "${DOCKPIPE_BIN:-}" \
    "$REPO_ROOT/src/bin/dockpipe" \
    "$(command -v dockpipe 2>/dev/null || true)"
  do
    if [[ -n "$dockpipe_bin" && -x "$dockpipe_bin" ]]; then
      # shellcheck disable=SC1090
      eval "$("$dockpipe_bin" sdk)"
      if declare -F dockpipe_sdk >/dev/null 2>&1; then
        dockpipe_sdk init-script >/dev/null 2>&1 || true
      fi
      return 0
    fi
  done
  return 1
}

dorkpipe_stack_detect_nvidia_gpu() {
  command -v nvidia-smi >/dev/null 2>&1 || return 1
  nvidia-smi -L >/dev/null 2>&1 || return 1
}

dorkpipe_stack_is_windows_host() {
  [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]] || [[ "${OSTYPE:-}" == msys* ]] || [[ "${OSTYPE:-}" == cygwin* ]] || [[ "${OSTYPE:-}" == win32 ]]
}

dorkpipe_stack_docker_supports_nvidia_gpu() {
  local docker_runtimes
  docker_runtimes="$(docker info --format '{{json .Runtimes}}' 2>/dev/null || true)"
  printf '%s' "$docker_runtimes" | grep -qi '"nvidia"'
}

dorkpipe_stack_wait_for_docker() {
  local attempts="${1:-30}"
  local i
  for ((i = 0; i < attempts; i++)); do
    if docker version >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

dorkpipe_stack_explain_docker_gpu_setup() {
  if dorkpipe_stack_is_windows_host; then
    cat >&2 <<'EOF'
dorkpipe-dev-stack: Docker GPU access is not enabled yet on this Windows host.
  Docker Desktop GPU support on Windows requires the WSL 2 backend.
  Required host setup:
    1. Install current NVIDIA Windows drivers with WSL 2 GPU support.
    2. Run: wsl.exe --update
    3. In Docker Desktop, enable "Use WSL 2 based engine".
    4. Make sure Docker Desktop is using Linux containers.
  Check: docker run --rm --gpus all ubuntu nvidia-smi
EOF
    return
  fi
  cat >&2 <<'EOF'
dorkpipe-dev-stack: Docker GPU access is not enabled yet.
  The host NVIDIA GPU is visible, but Docker does not report an nvidia runtime.
  To enable Ollama GPU access in containers, install/configure nvidia-container-toolkit,
  restart Docker, and rerun the stack.
  Check: docker run --rm --gpus all nvidia/cuda:12.4.1-base-ubuntu22.04 nvidia-smi
EOF
}

dorkpipe_stack_try_enable_windows_docker_gpu_access() {
  printf '[dorkpipe-dev-stack] Ollama GPU: detected Windows host; attempting WSL/Docker Desktop GPU prerequisites...\n' >&2
  if ! command -v wsl.exe >/dev/null 2>&1; then
    echo "dorkpipe-dev-stack: wsl.exe is not available on PATH, so Docker Desktop GPU setup cannot be automated here" >&2
    return 1
  fi
  if ! wsl.exe --update; then
    echo "dorkpipe-dev-stack: WSL update failed; Docker Desktop GPU support on Windows requires an updated WSL 2 kernel" >&2
    return 1
  fi
  printf '[dorkpipe-dev-stack] Ollama GPU: WSL update completed; verifying Docker GPU support again...\n' >&2
  if ! dorkpipe_stack_wait_for_docker 45; then
    echo "dorkpipe-dev-stack: Docker did not respond after the WSL update" >&2
    return 1
  fi
  if ! docker run --rm --gpus all ubuntu nvidia-smi >/dev/null 2>&1; then
    echo "dorkpipe-dev-stack: Docker Desktop still cannot run a GPU container after the WSL update" >&2
    echo "dorkpipe-dev-stack: open Docker Desktop Settings and enable the WSL 2 engine, then ensure Linux containers are active" >&2
    return 1
  fi
  return 0
}

dorkpipe_stack_write_gpu_setup_script() {
  local script_path="$1"
  cat > "$script_path" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ -f /etc/os-release ]]; then
  # shellcheck disable=SC1091
  . /etc/os-release
fi

if [[ "$(id -u)" -eq 0 ]]; then
  SUDO=""
else
  SUDO="sudo"
fi

ORIGINAL_USER="${DORKPIPE_GPU_SETUP_ORIGINAL_USER:-${SUDO_USER:-${USER:-}}}"
ORIGINAL_HOME="${DORKPIPE_GPU_SETUP_ORIGINAL_HOME:-${HOME:-}}"

log() {
  printf '[dorkpipe-dev-stack][gpu-setup] %s\n' "$*" >&2
}

run_as_original_user() {
  if [[ -n "$ORIGINAL_USER" && "$(id -u)" -eq 0 ]]; then
    if command -v sudo >/dev/null 2>&1; then
      sudo -u "$ORIGINAL_USER" HOME="$ORIGINAL_HOME" "$@"
      return $?
    fi
    if command -v su >/dev/null 2>&1; then
      su - "$ORIGINAL_USER" -c "$(printf '%q ' "$@")"
      return $?
    fi
  fi
  HOME="$ORIGINAL_HOME" "$@"
}

docker_is_rootless() {
  docker info --format '{{json .SecurityOptions}}' 2>/dev/null | grep -qi rootless
}

install_with_apt() {
  log "Detected apt-based system; installing NVIDIA Container Toolkit"
  $SUDO apt-get update
  $SUDO apt-get install -y --no-install-recommends curl gnupg2
  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | $SUDO gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
  curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
    sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
    $SUDO tee /etc/apt/sources.list.d/nvidia-container-toolkit.list >/dev/null
  $SUDO apt-get update
  $SUDO apt-get install -y nvidia-container-toolkit
}

install_with_dnf() {
  log "Detected dnf-based system; installing NVIDIA Container Toolkit"
  $SUDO dnf install -y curl
  curl -s -L https://nvidia.github.io/libnvidia-container/stable/rpm/nvidia-container-toolkit.repo | \
    $SUDO tee /etc/yum.repos.d/nvidia-container-toolkit.repo >/dev/null
  $SUDO dnf install -y nvidia-container-toolkit
}

install_with_yum() {
  log "Detected yum-based system; installing NVIDIA Container Toolkit"
  $SUDO yum install -y curl
  curl -s -L https://nvidia.github.io/libnvidia-container/stable/rpm/nvidia-container-toolkit.repo | \
    $SUDO tee /etc/yum.repos.d/nvidia-container-toolkit.repo >/dev/null
  $SUDO yum install -y nvidia-container-toolkit
}

install_with_zypper() {
  log "Detected zypper-based system; installing NVIDIA Container Toolkit"
  $SUDO zypper ar https://nvidia.github.io/libnvidia-container/stable/rpm/nvidia-container-toolkit.repo nvidia-container-toolkit
  $SUDO zypper --gpg-auto-import-keys install -y nvidia-container-toolkit
}

if command -v nvidia-ctk >/dev/null 2>&1; then
  :
elif command -v apt-get >/dev/null 2>&1; then
  install_with_apt
elif command -v dnf >/dev/null 2>&1; then
  install_with_dnf
elif command -v yum >/dev/null 2>&1; then
  install_with_yum
elif command -v zypper >/dev/null 2>&1; then
  install_with_zypper
else
  echo "dorkpipe-dev-stack: unsupported package manager for automatic Docker GPU setup" >&2
  exit 2
fi

if docker_is_rootless; then
  log "Detected rootless Docker; configuring user daemon"
  run_as_original_user nvidia-ctk runtime configure --runtime=docker --config="$ORIGINAL_HOME/.config/docker/daemon.json"
  run_as_original_user systemctl --user restart docker
  $SUDO nvidia-ctk config --set nvidia-container-cli.no-cgroups --in-place
else
  log "Detected rootful Docker; configuring system daemon"
  $SUDO nvidia-ctk runtime configure --runtime=docker
  $SUDO systemctl restart docker
fi
EOF
  chmod +x "$script_path"
}

dorkpipe_stack_run_gpu_setup_script() {
  local script_path="$1"
  if [[ -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]] && command -v pkexec >/dev/null 2>&1; then
    pkexec env PATH="$PATH" bash "$script_path"
    return $?
  fi
  if command -v sudo >/dev/null 2>&1; then
    sudo -v || return $?
    bash "$script_path"
    return $?
  fi
  echo "dorkpipe-dev-stack: automatic Docker GPU setup requires pkexec or sudo" >&2
  return 2
}

dorkpipe_stack_try_enable_docker_gpu_access() {
  if dorkpipe_stack_is_windows_host; then
    dorkpipe_stack_try_enable_windows_docker_gpu_access
    return $?
  fi
  local setup_script
  dorkpipe_stack_ensure_state_dir
  setup_script="$(mktemp "$(dorkpipe_stack_state_dir)/gpu-setup.XXXXXX.sh")"
  dorkpipe_stack_write_gpu_setup_script "$setup_script"
  if ! DORKPIPE_GPU_SETUP_ORIGINAL_USER="${SUDO_USER:-${USER:-}}" \
       DORKPIPE_GPU_SETUP_ORIGINAL_HOME="${HOME:-}" \
       dorkpipe_stack_run_gpu_setup_script "$setup_script"; then
    rm -f "$setup_script"
    return 1
  fi
  rm -f "$setup_script"
  if ! dorkpipe_stack_wait_for_docker 45; then
    echo "dorkpipe-dev-stack: Docker did not come back after GPU runtime configuration" >&2
    return 1
  fi
  if ! dorkpipe_stack_docker_supports_nvidia_gpu; then
    echo "dorkpipe-dev-stack: Docker restarted, but the nvidia runtime is still unavailable" >&2
    return 1
  fi
  printf '[dorkpipe-dev-stack] Ollama GPU: verifying Docker GPU access with a sample container...\n' >&2
  if ! docker run --rm --gpus all ubuntu nvidia-smi >/dev/null 2>&1; then
    echo "dorkpipe-dev-stack: Docker reports an nvidia runtime, but a sample GPU container still failed" >&2
    return 1
  fi
  return 0
}

dorkpipe_stack_write_gpu_override() {
  local gpu_file
  gpu_file="$(dorkpipe_stack_gpu_compose_file)"
  cat > "$gpu_file" <<'EOF'
services:
  ollama:
    gpus: all
    environment:
      NVIDIA_VISIBLE_DEVICES: all
      NVIDIA_DRIVER_CAPABILITIES: compute,utility
EOF
}

dorkpipe_stack_configure_gpu() {
  local requested mode_file gpu_file gpu_choice
  requested="${DORKPIPE_DEV_STACK_GPU:-auto}"
  mode_file="$(dorkpipe_stack_gpu_mode_file)"
  gpu_file="$(dorkpipe_stack_gpu_compose_file)"
  dorkpipe_stack_ensure_state_dir

  case "$requested" in
    auto|"")
      if dorkpipe_stack_detect_nvidia_gpu; then
        if dorkpipe_stack_docker_supports_nvidia_gpu; then
          dorkpipe_stack_write_gpu_override
          printf 'nvidia\n' > "$mode_file"
          printf '[dorkpipe-dev-stack] Ollama GPU: NVIDIA enabled via Docker Compose override\n' >&2
        else
          rm -f "$gpu_file"
          printf 'cpu\n' > "$mode_file"
          printf '[dorkpipe-dev-stack] Ollama GPU: host NVIDIA detected, but Docker GPU access is not ready yet\n' >&2
          printf '[dorkpipe-dev-stack] Ollama GPU: Docker GPU access needs nvidia-container-toolkit or equivalent runtime setup\n' >&2
          if declare -F dockpipe_sdk >/dev/null 2>&1; then
            gpu_choice="$(
              dockpipe_sdk prompt choice \
                --id dorkpipe_gpu_docker_access \
                --title "Docker GPU Access Is Not Enabled" \
                --message "DorkPipe found an NVIDIA GPU on the host, but Docker cannot use it yet. What would you like to do?" \
                --default "Continue with CPU for now" \
                --option "Enable Docker GPU access" \
                --option "Continue with CPU for now" \
                --option "Cancel launch"
            )" || return 1
            case "$gpu_choice" in
              "Enable Docker GPU access")
                printf '[dorkpipe-dev-stack] Ollama GPU: attempting to enable Docker GPU access...\n' >&2
                if dorkpipe_stack_try_enable_docker_gpu_access; then
                  dorkpipe_stack_write_gpu_override
                  printf 'nvidia\n' > "$mode_file"
                  printf '[dorkpipe-dev-stack] Ollama GPU: Docker GPU access enabled; continuing with NVIDIA\n' >&2
                else
                  dorkpipe_stack_explain_docker_gpu_setup
                  DORKPIPE_DEV_STACK_PROMPT_RESULT="gpu-setup"
                  return 0
                fi
                ;;
              "Continue with CPU for now")
                printf '[dorkpipe-dev-stack] Ollama GPU: continuing on CPU for this launch\n' >&2
                ;;
              *)
                DORKPIPE_DEV_STACK_PROMPT_RESULT="cancelled"
                return 0
                ;;
            esac
          else
            dorkpipe_stack_explain_docker_gpu_setup
            printf '[dorkpipe-dev-stack] Ollama GPU: continuing on CPU for this launch\n' >&2
            echo "dorkpipe-dev-stack: continuing on CPU because no interactive DockPipe SDK prompt was available" >&2
          fi
        fi
      else
        rm -f "$gpu_file"
        printf 'cpu\n' > "$mode_file"
        printf '[dorkpipe-dev-stack] Ollama GPU: no host NVIDIA GPU detected; using CPU\n' >&2
      fi
      ;;
    nvidia|all)
      dorkpipe_stack_write_gpu_override
      printf 'nvidia\n' > "$mode_file"
      printf '[dorkpipe-dev-stack] Ollama GPU: NVIDIA forced by DORKPIPE_DEV_STACK_GPU=%s\n' "$requested" >&2
      ;;
    cpu|none|off|0|false)
      rm -f "$gpu_file"
      printf 'cpu\n' > "$mode_file"
      printf '[dorkpipe-dev-stack] Ollama GPU: disabled by DORKPIPE_DEV_STACK_GPU=%s\n' "$requested" >&2
      ;;
    *)
      echo "dorkpipe-dev-stack: DORKPIPE_DEV_STACK_GPU must be auto, nvidia, all, cpu, none, off, 0, or false (got $requested)" >&2
      return 1
      ;;
  esac
}

dorkpipe_stack_compose_args() {
  local gpu_file
  gpu_file="$(dorkpipe_stack_gpu_compose_file)"
  printf '%s\n' -p "$PROJECT" -f "$COMPOSE"
  if [[ -s "$gpu_file" ]]; then
    printf '%s\n' -f "$gpu_file"
  fi
  printf '%s\n' --project-directory "$REPO_ROOT"
}
