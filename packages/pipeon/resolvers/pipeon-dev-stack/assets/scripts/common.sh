#!/usr/bin/env bash
set -euo pipefail

pipeon_stack_workdir() {
  local workdir="${DOCKPIPE_WORKDIR:-$PWD}"
  cd "$workdir" && pwd
}

pipeon_stack_repo_root() {
  if [[ -n "${PIPEON_DEV_STACK_PROJECT_DIR:-}" ]]; then
    cd "$PIPEON_DEV_STACK_PROJECT_DIR" && pwd
    return 0
  fi
  pipeon_stack_workdir
}

pipeon_stack_slug() {
  local workdir base
  workdir="$(pipeon_stack_workdir)"
  base="$(basename "$workdir")"
  printf '%s' "$base" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9' '-' | sed 's/^-*//; s/-*$//'
}

pipeon_stack_legacy_slug() {
  local workdir base
  workdir="$(pipeon_stack_workdir)"
  base="$(basename "$workdir")"
  printf '%s\n' "$base" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9' '-'
}

pipeon_stack_state_dir() {
  if [[ -n "${DOCKPIPE_PACKAGE_STATE_DIR:-}" ]]; then
    printf '%s\n' "$DOCKPIPE_PACKAGE_STATE_DIR"
    return 0
  fi
  local workdir
  workdir="$(pipeon_stack_workdir)"
  printf '%s/bin/.dockpipe/packages/pipeon-dev-stack\n' "$workdir"
}

pipeon_stack_compose_file() {
  local script_dir
  script_dir="$(dockpipe get script_dir)"
  printf '%s/../compose/docker-compose.yml\n' "$script_dir"
}

pipeon_stack_compose_project() {
  printf 'pipeon-dev-%s\n' "$(pipeon_stack_slug)"
}

pipeon_stack_legacy_compose_project() {
  printf 'pipeon-dev-%s\n' "$(pipeon_stack_legacy_slug)"
}

pipeon_stack_compose_network() {
  printf '%s_default\n' "$(pipeon_stack_compose_project)"
}

pipeon_stack_code_server_name() {
  printf 'pipeon-vscode-%s\n' "$(pipeon_stack_slug)"
}

pipeon_stack_legacy_code_server_name() {
  printf 'pipeon-vscode-%s\n' "$(pipeon_stack_legacy_slug)"
}

pipeon_stack_code_server_port_file() {
  printf '%s/code-server.port\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_pick_free_port() {
  python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

ensure_pipeon_stack_code_server_port() {
  local port_file port
  port_file="$(pipeon_stack_code_server_port_file)"
  ensure_pipeon_stack_state_dir
  if [[ -s "$port_file" ]]; then
    return 0
  fi
  port="$(pipeon_stack_pick_free_port)"
  printf '%s\n' "$port" > "$port_file"
}

pipeon_stack_code_server_port() {
  ensure_pipeon_stack_code_server_port
  tr -d ' \t\r\n' < "$(pipeon_stack_code_server_port_file)"
}

pipeon_stack_code_server_url() {
  printf 'http://127.0.0.1:%s/\n' "$(pipeon_stack_code_server_port)"
}

pipeon_stack_code_server_home() {
  printf '%s/code-server-home\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_desktop_bin() {
  local workdir repo_root candidate
  workdir="$(pipeon_stack_workdir)"
  repo_root="$(pipeon_stack_repo_root)"

  for candidate in \
    "$workdir/packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop" \
    "$repo_root/packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop"
  do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done

  command -v pipeon-desktop 2>/dev/null || true
}

pipeon_stack_mcp_port() {
  printf '%s\n' "${PIPEON_DEV_STACK_MCP_PORT:-8766}"
}

pipeon_stack_mcp_url() {
  printf 'http://127.0.0.1:%s/mcp\n' "$(pipeon_stack_mcp_port)"
}

pipeon_stack_mcp_container_url() {
  local network project container_name container_ip
  network="$(pipeon_stack_compose_network)"
  project="$(pipeon_stack_compose_project)"
  container_name="${project}-pipeon-mcp-proxy-1"
  container_ip="$(
    docker inspect "$container_name" \
      --format "{{with index .NetworkSettings.Networks \"$network\"}}{{.IPAddress}}{{end}}" \
      2>/dev/null || true
  )"
  if [[ -n "$container_ip" ]]; then
    printf 'http://%s:8766/mcp\n' "$container_ip"
    return 0
  fi
  printf 'http://host.docker.internal:%s/mcp\n' "$(pipeon_stack_mcp_port)"
}

pipeon_stack_api_key_file() {
  printf '%s/api-key\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_mcp_tls_cert_file() {
  printf '%s/mcp-tls.crt\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_mcp_tls_key_file() {
  printf '%s/mcp-tls.key\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_runtime_env() {
  printf '%s/runtime.env\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_gpu_compose_file() {
  printf '%s/docker-compose.gpu.yml\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_gpu_mode_file() {
  printf '%s/gpu.mode\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_image_stamp_file() {
  printf '%s/code-server-image.stamp\n' "$(pipeon_stack_state_dir)"
}

ensure_pipeon_stack_state_dir() {
  mkdir -p "$(pipeon_stack_state_dir)"
}

ensure_pipeon_stack_api_key() {
  local key_file
  key_file="$(pipeon_stack_api_key_file)"
  ensure_pipeon_stack_state_dir
  if [[ -s "$key_file" ]]; then
    return 0
  fi
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 24 > "$key_file"
  else
    date +%s%N | sha256sum | cut -d' ' -f1 > "$key_file"
  fi
  chmod 600 "$key_file" 2>/dev/null || true
}

ensure_pipeon_stack_mcp_tls_material() {
  local cert_file key_file
  cert_file="$(pipeon_stack_mcp_tls_cert_file)"
  key_file="$(pipeon_stack_mcp_tls_key_file)"
  ensure_pipeon_stack_state_dir
  if [[ -s "$cert_file" && -s "$key_file" ]]; then
    return 0
  fi
  if ! command -v openssl >/dev/null 2>&1; then
    echo "pipeon-dev-stack: openssl is required to generate local MCP TLS material" >&2
    exit 1
  fi
  openssl req -x509 -nodes -newkey rsa:2048 \
    -keyout "$key_file" \
    -out "$cert_file" \
    -days 365 \
    -subj "/CN=dorkpipe-stack" \
    -addext "subjectAltName=DNS:dorkpipe-stack,DNS:localhost,IP:127.0.0.1" >/dev/null 2>&1
  chmod 600 "$key_file" 2>/dev/null || true
  chmod 644 "$cert_file" 2>/dev/null || true
}

pipeon_stack_detect_nvidia_gpu() {
  command -v nvidia-smi >/dev/null 2>&1 || return 1
  nvidia-smi -L >/dev/null 2>&1 || return 1
}

pipeon_stack_is_windows_host() {
  [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]] || [[ "${OSTYPE:-}" == msys* ]] || [[ "${OSTYPE:-}" == cygwin* ]] || [[ "${OSTYPE:-}" == win32 ]]
}

pipeon_stack_docker_supports_nvidia_gpu() {
  local docker_runtimes
  docker_runtimes="$(docker info --format '{{json .Runtimes}}' 2>/dev/null || true)"
  printf '%s' "$docker_runtimes" | grep -qi '"nvidia"'
}

pipeon_stack_explain_docker_gpu_setup() {
  if pipeon_stack_is_windows_host; then
    cat >&2 <<'EOF'
pipeon-dev-stack: Docker GPU access is not enabled yet on this Windows host.
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
pipeon-dev-stack: Docker GPU access is not enabled yet.
  The host NVIDIA GPU is visible, but Docker does not report an nvidia runtime.
  To enable Ollama GPU access in containers, install/configure nvidia-container-toolkit,
  restart Docker, and relaunch the stack.
  Check: docker run --rm --gpus all nvidia/cuda:12.4.1-base-ubuntu22.04 nvidia-smi
EOF
}

pipeon_stack_try_enable_windows_docker_gpu_access() {
  printf '[pipeon-dev-stack] Ollama GPU: detected Windows host; attempting WSL/Docker Desktop GPU prerequisites...\n' >&2
  if ! command -v wsl.exe >/dev/null 2>&1; then
    echo "pipeon-dev-stack: wsl.exe is not available on PATH, so Docker Desktop GPU setup cannot be automated here" >&2
    return 1
  fi
  if ! wsl.exe --update; then
    echo "pipeon-dev-stack: WSL update failed; Docker Desktop GPU support on Windows requires an updated WSL 2 kernel" >&2
    return 1
  fi
  printf '[pipeon-dev-stack] Ollama GPU: WSL update completed; verifying Docker GPU support again...\n' >&2
  if ! pipeon_stack_wait_for_docker 45; then
    echo "pipeon-dev-stack: Docker did not respond after the WSL update" >&2
    return 1
  fi
  if ! docker run --rm --gpus all ubuntu nvidia-smi >/dev/null 2>&1; then
    echo "pipeon-dev-stack: Docker Desktop still cannot run a GPU container after the WSL update" >&2
    echo "pipeon-dev-stack: open Docker Desktop Settings and enable the WSL 2 engine, then ensure Linux containers are active" >&2
    return 1
  fi
  return 0
}

pipeon_stack_wait_for_docker() {
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

pipeon_stack_write_gpu_setup_script() {
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

ORIGINAL_USER="${PIPEON_GPU_SETUP_ORIGINAL_USER:-${SUDO_USER:-${USER:-}}}"
ORIGINAL_HOME="${PIPEON_GPU_SETUP_ORIGINAL_HOME:-${HOME:-}}"

log() {
  printf '[pipeon-dev-stack][gpu-setup] %s\n' "$*" >&2
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
  echo "pipeon-dev-stack: unsupported package manager for automatic Docker GPU setup" >&2
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

pipeon_stack_run_gpu_setup_script() {
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
  echo "pipeon-dev-stack: automatic Docker GPU setup requires pkexec or sudo" >&2
  return 2
}

pipeon_stack_try_enable_docker_gpu_access() {
  if pipeon_stack_is_windows_host; then
    pipeon_stack_try_enable_windows_docker_gpu_access
    return $?
  fi
  local setup_script
  setup_script="$(mktemp "$(pipeon_stack_state_dir)/gpu-setup.XXXXXX.sh")"
  pipeon_stack_write_gpu_setup_script "$setup_script"
  if ! PIPEON_GPU_SETUP_ORIGINAL_USER="${SUDO_USER:-${USER:-}}" \
       PIPEON_GPU_SETUP_ORIGINAL_HOME="${HOME:-}" \
       pipeon_stack_run_gpu_setup_script "$setup_script"; then
    rm -f "$setup_script"
    return 1
  fi
  rm -f "$setup_script"
  if ! pipeon_stack_wait_for_docker 45; then
    echo "pipeon-dev-stack: Docker did not come back after GPU runtime configuration" >&2
    return 1
  fi
  if ! pipeon_stack_docker_supports_nvidia_gpu; then
    echo "pipeon-dev-stack: Docker restarted, but the nvidia runtime is still unavailable" >&2
    return 1
  fi
  printf '[pipeon-dev-stack] Ollama GPU: verifying Docker GPU access with a sample container...\n' >&2
  if ! docker run --rm --gpus all ubuntu nvidia-smi >/dev/null 2>&1; then
    echo "pipeon-dev-stack: Docker reports an nvidia runtime, but a sample GPU container still failed" >&2
    return 1
  fi
  return 0
}

write_pipeon_stack_gpu_override() {
  local gpu_file
  gpu_file="$(pipeon_stack_gpu_compose_file)"
  cat > "$gpu_file" <<'EOF'
services:
  ollama:
    gpus: all
    environment:
      NVIDIA_VISIBLE_DEVICES: all
      NVIDIA_DRIVER_CAPABILITIES: compute,utility
EOF
}

configure_pipeon_stack_gpu() {
  local requested mode_file gpu_file
  requested="${PIPEON_DEV_STACK_GPU:-auto}"
  mode_file="$(pipeon_stack_gpu_mode_file)"
  gpu_file="$(pipeon_stack_gpu_compose_file)"
  ensure_pipeon_stack_state_dir

  case "$requested" in
    auto|"")
      if pipeon_stack_detect_nvidia_gpu; then
        if pipeon_stack_docker_supports_nvidia_gpu; then
          write_pipeon_stack_gpu_override
          printf 'nvidia\n' > "$mode_file"
          printf '[pipeon-dev-stack] Ollama GPU: NVIDIA enabled via Docker Compose override\n' >&2
        else
          rm -f "$gpu_file"
          printf 'cpu\n' > "$mode_file"
          printf '[pipeon-dev-stack] Ollama GPU: host NVIDIA detected, but Docker GPU access is not ready yet\n' >&2
          printf '[pipeon-dev-stack] Ollama GPU: Docker GPU access needs nvidia-container-toolkit or equivalent runtime setup\n' >&2
          if declare -F dockpipe_sdk >/dev/null 2>&1; then
            local gpu_choice
            gpu_choice="$(
              dockpipe_sdk prompt choice \
                --id pipeon_gpu_docker_access \
                --title "Docker GPU Access Is Not Enabled" \
                --message "Ollama found an NVIDIA GPU on the host, but Docker cannot use it yet. What would you like to do?" \
                --default "Continue with CPU for now" \
                --option "Enable Docker GPU access" \
                --option "Continue with CPU for now" \
                --option "Cancel launch"
            )" || return 1
            case "$gpu_choice" in
              "Enable Docker GPU access")
                printf '[pipeon-dev-stack] Ollama GPU: attempting to enable Docker GPU access...\n' >&2
                if pipeon_stack_try_enable_docker_gpu_access; then
                  write_pipeon_stack_gpu_override
                  printf 'nvidia\n' > "$mode_file"
                  printf '[pipeon-dev-stack] Ollama GPU: Docker GPU access enabled; continuing with NVIDIA\n' >&2
                else
                  pipeon_stack_explain_docker_gpu_setup
                  PIPEON_DEV_STACK_PROMPT_RESULT="gpu-setup"
                  return 0
                fi
                ;;
              "Continue with CPU for now")
                printf '[pipeon-dev-stack] Ollama GPU: continuing on CPU for this launch\n' >&2
                :
                ;;
              *)
                PIPEON_DEV_STACK_PROMPT_RESULT="cancelled"
                return 0
                ;;
            esac
          else
            pipeon_stack_explain_docker_gpu_setup
            printf '[pipeon-dev-stack] Ollama GPU: continuing on CPU for this launch\n' >&2
            echo "pipeon-dev-stack: continuing on CPU because no interactive DockPipe SDK prompt was available" >&2
          fi
        fi
      else
        rm -f "$gpu_file"
        printf 'cpu\n' > "$mode_file"
        printf '[pipeon-dev-stack] Ollama GPU: no host NVIDIA GPU detected; using CPU\n' >&2
      fi
      ;;
    nvidia|all)
      write_pipeon_stack_gpu_override
      printf 'nvidia\n' > "$mode_file"
      printf '[pipeon-dev-stack] Ollama GPU: NVIDIA forced by PIPEON_DEV_STACK_GPU=%s\n' "$requested" >&2
      ;;
    cpu|none|off|0|false)
      rm -f "$gpu_file"
      printf 'cpu\n' > "$mode_file"
      printf '[pipeon-dev-stack] Ollama GPU: disabled by PIPEON_DEV_STACK_GPU=%s\n' "$requested" >&2
      ;;
    *)
      echo "pipeon-dev-stack: PIPEON_DEV_STACK_GPU must be auto, nvidia, all, cpu, none, off, 0, or false (got $requested)" >&2
      exit 1
      ;;
  esac
}

pipeon_stack_gpu_mode() {
  local mode_file
  mode_file="$(pipeon_stack_gpu_mode_file)"
  if [[ -s "$mode_file" ]]; then
    printf '%s\n' "$(tr -d ' \t\r\n' < "$mode_file")"
    return 0
  fi
  printf 'unknown\n'
}

pipeon_stack_gpu_status_file() {
  printf '%s/gpu.status\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_gpu_status() {
  local status_file
  status_file="$(pipeon_stack_gpu_status_file)"
  if [[ -s "$status_file" ]]; then
    tr '\n' ' ' < "$status_file" | sed 's/[[:space:]]*$//'
    printf '\n'
    return 0
  fi
  printf 'unknown\n'
}

verify_pipeon_stack_ollama_gpu() {
  local mode status_file
  mode="$(pipeon_stack_gpu_mode)"
  status_file="$(pipeon_stack_gpu_status_file)"
  case "$mode" in
    nvidia)
      if compose_cmd exec -T ollama sh -lc 'test -e /dev/nvidiactl || test -e /dev/nvidia0 || test -d /proc/driver/nvidia' >/dev/null 2>&1; then
        printf 'nvidia-attached\n' > "$status_file"
        printf '[pipeon-dev-stack] Ollama GPU: NVIDIA devices attached to container\n' >&2
        return 0
      fi
      printf 'nvidia-requested-but-not-attached\n' > "$status_file"
      echo "pipeon-dev-stack: NVIDIA GPU was requested, but the Ollama container has no NVIDIA devices" >&2
      echo "  Install/configure nvidia-container-toolkit, then restart Docker and relaunch the stack." >&2
      echo "  Check: docker run --rm --gpus all nvidia/cuda:12.4.1-base-ubuntu22.04 nvidia-smi" >&2
      return 1
      ;;
    cpu)
      printf 'cpu\n' > "$status_file"
      ;;
    *)
      printf 'unknown\n' > "$status_file"
      ;;
  esac
}

pipeon_stack_compose_base_args() {
  local gpu_file
  gpu_file="$(pipeon_stack_gpu_compose_file)"
  printf '%s\n' --env-file "$RUNTIME_ENV" -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE"
  if [[ -s "$gpu_file" ]]; then
    printf '%s\n' -f "$gpu_file"
  fi
  printf '%s\n' --project-directory "$PROJECT_DIR"
}

write_pipeon_stack_runtime_env() {
  local workdir repo_root api_key_file tls_cert_file tls_key_file
  workdir="$(pipeon_stack_workdir)"
  repo_root="$(pipeon_stack_repo_root)"
  api_key_file="$(pipeon_stack_api_key_file)"
  tls_cert_file="$(pipeon_stack_mcp_tls_cert_file)"
  tls_key_file="$(pipeon_stack_mcp_tls_key_file)"
  cat > "$(pipeon_stack_runtime_env)" <<EOF
WORKDIR=$workdir
REPO_ROOT=$repo_root
PIPEON_DEV_STACK_WORKDIR=$workdir
PIPEON_DEV_STACK_REPO_ROOT=$repo_root
PIPEON_DEV_STACK_MCP_PORT=$(pipeon_stack_mcp_port)
PIPEON_DEV_STACK_MCP_API_KEY_FILE=$api_key_file
PIPEON_DEV_STACK_MCP_TLS_CERT_FILE=$tls_cert_file
PIPEON_DEV_STACK_MCP_TLS_KEY_FILE=$tls_key_file
PIPEON_DEV_STACK_DOCKPIPE_BIN=/repo/src/bin/dockpipe
PIPEON_DEV_STACK_DORKPIPE_BIN=/repo/packages/dorkpipe/bin/dorkpipe
PIPEON_DEV_STACK_MCPD_BIN=/repo/packages/dorkpipe/bin/mcpd
PIPEON_DEV_STACK_DORKPIPE_WORKDIR=/work
PIPEON_DEV_STACK_DORKPIPE_DATABASE_URL=postgresql://dorkpipe:dorkpipe@postgres:5432/dorkpipe
PIPEON_DEV_STACK_DORKPIPE_OLLAMA_HOST=http://ollama:11434
DORKPIPE_DEV_STACK_PROJECT=$(pipeon_stack_compose_project)
DORKPIPE_DEV_STACK_NETWORK=$(pipeon_stack_compose_network)
CODE_SERVER_CONTAINER_NAME=$(pipeon_stack_code_server_name)
CODE_SERVER_URL=$(pipeon_stack_code_server_url)
MCP_HTTP_URL=$(pipeon_stack_mcp_url)
MCP_HTTP_CONTAINER_URL=$(pipeon_stack_mcp_container_url)
MCP_HTTP_API_KEY_FILE=$api_key_file
PIPEON_DEV_STACK_GPU=${PIPEON_DEV_STACK_GPU:-auto}
EOF
}

pipeon_stack_compose_running() {
  local compose_file project
  compose_file="$(pipeon_stack_compose_file)"
  project="$(pipeon_stack_compose_project)"
  docker compose -p "$project" -f "$compose_file" ps -q 2>/dev/null | grep -q .
}
