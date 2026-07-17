#!/usr/bin/env bash
set -euo pipefail

resolve_sdk_dockpipe_bin() {
  local candidate
  for candidate in \
    "${DOCKPIPE_BIN:-}" \
    "${DOCKPIPE_WORKDIR:-}/src/bin/dockpipe" \
    "${DOCKPIPE_WORKDIR:-}/src/bin/dockpipe.exe" \
    "$(pwd)/src/bin/dockpipe" \
    "$(pwd)/src/bin/dockpipe.exe" \
    "$(command -v dockpipe 2>/dev/null || true)"
  do
    if [[ -n "$candidate" && -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

DOCKPIPE_SDK_BIN="$(resolve_sdk_dockpipe_bin)" || {
  echo "pipeon-dev-stack: dockpipe binary not found for SDK bootstrap" >&2
  exit 1
}

eval "$("$DOCKPIPE_SDK_BIN" sdk)"
dockpipe_sdk init-script

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

DOCKPIPE_HOST_BASH_BIN="$(resolve_host_bash_bin)" || {
  echo "pipeon-dev-stack: host bash executable not found after SDK bootstrap" >&2
  exit 1
}
export DOCKPIPE_HOST_BASH_BIN

report_launch_failure() {
  local exit_code="$1"
  local line_no="$2"
  local command="$3"
  printf 'pipeon-dev-stack: launch.sh failed at line %s while running: %s (exit %s)\n' \
    "$line_no" "$command" "$exit_code" >&2
}

trap 'report_launch_failure "$?" "${LINENO}" "${BASH_COMMAND}"' ERR
# shellcheck source=/dev/null
source "$SCRIPT_DIR/common.sh"

WORKDIR="$(pipeon_stack_workdir)"
PROJECT_DIR="$(pipeon_stack_repo_root)"
COMPOSE_FILE="$(pipeon_stack_compose_file)"
COMPOSE_PROJECT="$(pipeon_stack_compose_project)"
LEGACY_COMPOSE_PROJECT="$(pipeon_stack_legacy_compose_project)"
CODE_SERVER_CONTAINER_NAME="$(pipeon_stack_code_server_name)"
LEGACY_CODE_SERVER_CONTAINER_NAME="$(pipeon_stack_legacy_code_server_name)"
CODE_SERVER_URL="$(pipeon_stack_code_server_url)"
MCP_PORT="$(pipeon_stack_mcp_port)"
MCP_URL="$(pipeon_stack_mcp_url)"
RUNTIME_ENV="$(pipeon_stack_runtime_env)"
AUTODOWN="${PIPEON_DEV_STACK_AUTODOWN:-1}"
BUILD_MODE="${PIPEON_DEV_STACK_BUILD:-auto}"
MODEL_NAME="${PIPEON_OLLAMA_MODEL:-${DOCKPIPE_OLLAMA_MODEL:-llama3.2}}"
PIPEON_DESKTOP_BIN="${PIPEON_DESKTOP_BIN:-$(pipeon_stack_desktop_bin)}"
PIPEON_DESKTOP_SCRIPT="$SCRIPT_DIR/desktop.sh"
CODE_SERVER_IMAGE="${CODE_SERVER_IMAGE:-dockpipe-code-server:latest}"
COMPOSE_ASSETS_DIR="$(cd "$SCRIPT_DIR/../compose" && pwd)"

is_windows_host() {
  pipeon_stack_is_windows_host
}

resolve_tool_bin() {
  local configured="${1:-}"
  local command_name="$2"
  if [[ -n "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi
  if command -v "$command_name" >/dev/null 2>&1; then
    command -v "$command_name"
    return 0
  fi
  return 1
}

resolve_repo_tool_bin() {
  local configured="${1:-}"
  local command_name="$2"
  shift 2
  local candidate

  if [[ -n "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi

  for candidate in "$@"; do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
    if is_windows_host && [[ -f "$candidate.exe" ]]; then
      printf '%s\n' "$candidate.exe"
      return 0
    fi
  done

  resolve_tool_bin "" "$command_name"
}

resolve_pipeon_bin() {
  local configured="${1:-}"
  local candidate
  if [[ -n "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi

  for candidate in \
    "$WORKDIR/packages/pipeon/resolvers/pipeon/bin/pipeon" \
    "$PROJECT_DIR/packages/pipeon/resolvers/pipeon/bin/pipeon"
  do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done

  resolve_tool_bin "" pipeon
}

ensure_executable_binary() {
  local path="$1"
  if [[ ! -e "$path" ]]; then
    return 1
  fi
  if [[ -x "$path" ]]; then
    return 0
  fi
  chmod +x "$path" 2>/dev/null || true
  [[ -x "$path" ]]
}

PIPEON_DEV_STACK_LINUX_TOOLING_CHANGED=0
PIPEON_DEV_STACK_REBUILD_STACK_IMAGE=0
PIPEON_DEV_STACK_STACK_IMAGE_SIGNATURE=""

pipeon_stack_dorkpipe_stack_image_stamp_file() {
  printf '%s/dorkpipe-stack-image.stamp\n' "$(pipeon_stack_state_dir)"
}

pipeon_stack_linux_tool_is_current() {
  local module_dir="$1"
  local output="$2"
  [[ -f "$output" ]] || return 1
  if [[ -f "$PROJECT_DIR/VERSION" && "$PROJECT_DIR/VERSION" -nt "$output" ]]; then
    return 1
  fi
  ! find "$module_dir" \
    \( -path '*/.git' -o -path '*/bin' -o -path '*/node_modules' -o -path '*/.dockpipe' \) -prune \
    -o -type f \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) -newer "$output" -print -quit \
    | grep -q .
}

build_pipeon_stack_linux_tool() {
  local label="$1"
  local module_dir="$2"
  local package_path="$3"
  local output="$4"
  local version="${5:-0.0.0}"

  if ! command -v go >/dev/null 2>&1; then
    echo "pipeon-dev-stack: Go is required to build Linux binaries for the DorkPipe stack image" >&2
    return 1
  fi
  if [[ ! -d "$module_dir" ]]; then
    echo "pipeon-dev-stack: missing source directory for $label: $module_dir" >&2
    return 1
  fi

  mkdir -p "$(dirname "$output")" "$(pipeon_stack_state_dir)/linux-go-cache" "$(pipeon_stack_state_dir)/linux-go-tmp"
  if pipeon_stack_linux_tool_is_current "$module_dir" "$output"; then
    printf '[pipeon-dev-stack] using cached Linux stack binary: %s -> %s\n' "$label" "$output" >&2
    return 0
  fi

  printf '[pipeon-dev-stack] building Linux stack binary: %s -> %s\n' "$label" "$output" >&2
  (
    cd "$module_dir"
    GOOS="${PIPEON_DEV_STACK_GOOS:-linux}" \
    GOARCH="${PIPEON_DEV_STACK_GOARCH:-amd64}" \
    CGO_ENABLED="${PIPEON_DEV_STACK_CGO_ENABLED:-0}" \
    GOCACHE="${GOCACHE:-$(pipeon_stack_state_dir)/linux-go-cache}" \
    GOTMPDIR="${GOTMPDIR:-$(pipeon_stack_state_dir)/linux-go-tmp}" \
      go build -trimpath -ldflags "-s -w -X main.Version=${version}" -o "$output" "$package_path"
  )
  chmod +x "$output"
  PIPEON_DEV_STACK_LINUX_TOOLING_CHANGED=1
}

prepare_pipeon_stack_linux_binaries() {
  local output_dir version
  output_dir="$(pipeon_stack_state_dir)/linux-tooling/bin"
  version="0.0.0"
  if [[ -f "$PROJECT_DIR/VERSION" ]]; then
    version="$(tr -d ' \t\r\n' < "$PROJECT_DIR/VERSION")"
  fi

  if [[ -f "$PROJECT_DIR/VERSION" && -d "$PROJECT_DIR/src/cmd" ]]; then
    cp "$PROJECT_DIR/VERSION" "$PROJECT_DIR/src/cmd/VERSION"
  fi

  build_pipeon_stack_linux_tool \
    dockpipe \
    "$PROJECT_DIR" \
    ./src/cmd \
    "$output_dir/dockpipe" \
    "$version"
  build_pipeon_stack_linux_tool \
    dorkpipe \
    "$PROJECT_DIR/packages/dorkpipe/lib" \
    ./cmd/dorkpipe \
    "$output_dir/dorkpipe" \
    "$version"
  build_pipeon_stack_linux_tool \
    mcpd \
    "$PROJECT_DIR/packages/dorkpipe/mcp" \
    ./cmd/mcpd \
    "$output_dir/mcpd" \
    "$version"
}

prepare_pipeon_stack_context() {
  local context_root
  context_root="$(pipeon_stack_context_dir)"
  prepare_pipeon_stack_linux_binaries
  rm -rf "$context_root"
  mkdir -p "$context_root/compose" "$context_root/tooling/bin/linux"
  cp "$COMPOSE_ASSETS_DIR/Dockerfile.dorkpipe-stack" "$context_root/compose/Dockerfile.dorkpipe-stack"
  if [[ -f "$COMPOSE_ASSETS_DIR/Dockerfile.dorkpipe-stack.dockerignore" ]]; then
    cp "$COMPOSE_ASSETS_DIR/Dockerfile.dorkpipe-stack.dockerignore" "$context_root/compose/Dockerfile.dorkpipe-stack.dockerignore"
  fi
  cp "$(pipeon_stack_state_dir)/linux-tooling/bin/dockpipe" "$context_root/tooling/bin/linux/dockpipe"
  cp "$(pipeon_stack_state_dir)/linux-tooling/bin/dorkpipe" "$context_root/tooling/bin/linux/dorkpipe"
  cp "$(pipeon_stack_state_dir)/linux-tooling/bin/mcpd" "$context_root/tooling/bin/linux/mcpd"
  chmod +x \
    "$context_root/tooling/bin/linux/dockpipe" \
    "$context_root/tooling/bin/linux/dorkpipe" \
    "$context_root/tooling/bin/linux/mcpd"

  local stamp_file saved_sig current_sig have_image
  stamp_file="$(pipeon_stack_dorkpipe_stack_image_stamp_file)"
  current_sig=""
  if command -v sha256sum >/dev/null 2>&1; then
    current_sig="$(
      cd "$context_root"
      sha256sum \
        compose/Dockerfile.dorkpipe-stack \
        tooling/bin/linux/dockpipe \
        tooling/bin/linux/dorkpipe \
        tooling/bin/linux/mcpd 2>/dev/null | sha256sum | awk '{print $1}'
    )"
  fi
  saved_sig="$(cat "$stamp_file" 2>/dev/null || true)"
  have_image=0
  docker image inspect dockpipe-dorkpipe-stack:latest >/dev/null 2>&1 && have_image=1
  PIPEON_DEV_STACK_STACK_IMAGE_SIGNATURE="$current_sig"
  if [[ "$have_image" -eq 0 || -z "$current_sig" || "$current_sig" != "$saved_sig" || "$PIPEON_DEV_STACK_LINUX_TOOLING_CHANGED" == "1" ]]; then
    PIPEON_DEV_STACK_REBUILD_STACK_IMAGE=1
  fi
}

compose_cmd() {
  local args=()
  mapfile -t args < <(pipeon_stack_compose_base_args)
  docker compose "${args[@]}" "$@"
}

retry_with_backoff() {
  local label="$1"
  local attempts="$2"
  local delay_seconds="$3"
  shift 3

  local attempt=1
  local status=0
  while true; do
    if "$@"; then
      return 0
    fi
    status=$?
    if (( attempt >= attempts )); then
      printf '[pipeon-dev-stack] %s failed after %s attempt(s)\n' "$label" "$attempt" >&2
      return "$status"
    fi
    printf '[pipeon-dev-stack] %s failed on attempt %s/%s; retrying in %ss\n' \
      "$label" "$attempt" "$attempts" "$delay_seconds" >&2
    sleep "$delay_seconds"
    attempt=$((attempt + 1))
  done
}

ensure_pipeon_code_server_surface() {
  if [[ "$CODE_SERVER_IMAGE" != "dockpipe-code-server:latest" ]]; then
    return 0
  fi

  local build_script stamp_file current_sig saved_sig have_image refresh_reason
  build_script="$(pipeon_stack_build_script)"
  stamp_file="$(pipeon_stack_image_stamp_file)"
  current_sig="$(pipeon_stack_code_server_image_signature)"
  saved_sig="$(cat "$stamp_file" 2>/dev/null || true)"
  have_image=0
  docker image inspect dockpipe-code-server:latest >/dev/null 2>&1 && have_image=1

  case "$BUILD_MODE" in
    always)
      refresh_reason="forced by PIPEON_DEV_STACK_BUILD=always"
      ;;
    auto)
      if [[ "$have_image" -eq 0 ]]; then
        refresh_reason="Pipeon code-server image is missing"
      elif [[ "$saved_sig" != "$current_sig" ]]; then
        refresh_reason="Pipeon-managed code-server inputs changed"
      else
        refresh_reason=""
      fi
      ;;
    never)
      refresh_reason=""
      ;;
  esac

  if [[ -z "$refresh_reason" ]]; then
    return 0
  fi

  if [[ ! -x "$build_script" && ! -f "$build_script" ]]; then
    echo "pipeon-dev-stack: missing Pipeon build helper at $build_script" >&2
    return 1
  fi

  printf '[pipeon-dev-stack] refreshing Pipeon code-server surface: %s\n' "$refresh_reason" >&2
  printf '[pipeon-dev-stack] invoking build helper: %s code-server-image\n' "$build_script" >&2
  local build_status=0
  (
    cd "$PROJECT_DIR"
    DOCKPIPE_WORKDIR="$WORKDIR" "$DOCKPIPE_HOST_BASH_BIN" "$build_script" code-server-image
  )
  build_status=$?
  if [[ "$build_status" -eq 0 ]]; then
    printf '[pipeon-dev-stack] build helper returned successfully\n' >&2
    ensure_pipeon_stack_state_dir
    printf '%s\n' "$current_sig" > "$stamp_file"
    return 0
  fi
  printf '[pipeon-dev-stack] build helper exited with status %s\n' "$build_status" >&2

  if [[ "$have_image" -eq 1 ]]; then
    printf '[pipeon-dev-stack] refresh failed; continuing with the existing dockpipe-code-server image.\n' >&2
    return 0
  fi

  echo "pipeon-dev-stack: could not build the managed Pipeon code-server image and no existing image is available" >&2
  return 1
}

wait_for_ollama_ready() {
  local attempts="${1:-60}"
  local i
  for ((i = 0; i < attempts; i++)); do
    if compose_cmd exec -T ollama ollama list >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

ollama_model_available() {
  local model="$1"
  local normalized_model names
  normalized_model="${model%:latest}"
  names="$(
    compose_cmd exec -T ollama ollama list 2>/dev/null \
      | awk 'NR > 1 {print $1}' \
      | sed 's/:latest$//'
  )"
  printf '%s\n' "$names" | grep -Fxq "$normalized_model"
}

wait_for_mcp_ready() {
  local attempts="${2:-40}"
  local i code
  for ((i = 0; i < attempts; i++)); do
    code="$(
      curl -sS -o /dev/null -w '%{http_code}' \
        "$MCP_URL" 2>/dev/null || true
    )"
    case "$code" in
      200|204|400|401|405)
        return 0
        ;;
    esac
    sleep 0.25
  done
  return 1
}

wait_for_host_mcp_ready() {
  local url attempts i code
  url="$(pipeon_stack_host_mcp_url)"
  attempts="${1:-40}"
  for ((i = 0; i < attempts; i++)); do
    code="$(
      curl -sS -o /dev/null -w '%{http_code}' \
        "$url" 2>/dev/null || true
    )"
    case "$code" in
      200|204|400|401|405)
        return 0
        ;;
    esac
    sleep 0.25
  done
  return 1
}

pipeon_host_mcp_has_required_tools() {
  local url api_key_file api_key response
  url="$(pipeon_stack_host_mcp_url)"
  api_key_file="$(pipeon_stack_host_mcp_api_key_file)"
  [[ -s "$api_key_file" ]] || return 1
  api_key="$(tr -d ' \t\r\n' < "$api_key_file")"
  [[ -n "$api_key" ]] || return 1
  response="$(
    curl -sS \
      -H "Authorization: Bearer ${api_key}" \
      -H "Content-Type: application/json" \
      --data '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' \
      "$url" 2>/dev/null || true
  )"
  [[ "$response" == *'"dorkpipe.provider_pool_catalog"'* ]] || return 1
  [[ "$response" == *'"dorkpipe.provider_pool_status"'* ]] || return 1
  [[ "$response" == *'"dorkpipe.provider_pool_chat"'* ]] || return 1
  [[ "$response" == *'"dorkpipe.host_codex_chat"'* ]] || return 1
  [[ "$response" == *'"dorkpipe.host_claude_chat"'* ]] || return 1
  [[ "$response" == *'"dorkpipe.host_claude_auth"'* ]] || return 1
  [[ "$response" == *'"dorkpipe.provider_auth_status"'* ]] || return 1
  [[ "$response" == *'"dorkpipe.provider_auth_repair"'* ]] || return 1
}

pipeon_host_mcp_pid_running() {
  local pid_file pid
  pid_file="$(pipeon_stack_host_mcp_pid_file)"
  [[ -s "$pid_file" ]] || return 1
  pid="$(tr -d ' \t\r\n' < "$pid_file")"
  [[ -n "$pid" ]] || return 1
  kill -0 "$pid" >/dev/null 2>&1
}

stop_pipeon_host_mcp_bridge() {
  local pid_file pid port powershell_bin
  pid_file="$(pipeon_stack_host_mcp_pid_file)"
  if [[ -s "$pid_file" ]]; then
    pid="$(tr -d ' \t\r\n' < "$pid_file")"
  fi
  if [[ -n "${pid:-}" ]]; then
    kill "$pid" >/dev/null 2>&1 || true
  fi
  if pipeon_stack_is_windows_host; then
    port="$(pipeon_stack_host_mcp_port)"
    powershell_bin="$(pipeon_stack_powershell_bin 2>/dev/null || true)"
    if [[ -n "$powershell_bin" ]]; then
      PIPEON_HOST_MCP_PORT="$port" pipeon_stack_powershell_hidden "$powershell_bin" -Command '
        $portValue = [int]$env:PIPEON_HOST_MCP_PORT
        Get-NetTCPConnection -LocalAddress 127.0.0.1 -LocalPort $portValue -State Listen -ErrorAction SilentlyContinue |
          Select-Object -ExpandProperty OwningProcess -Unique |
          ForEach-Object {
            try { Stop-Process -Id $_ -Force -ErrorAction Stop } catch {}
          }
      ' >/dev/null 2>&1 || true
    fi
  fi
  rm -f "$pid_file"
}

start_pipeon_host_mcp_bridge() {
  local api_key_file pid_file port log_file
  api_key_file="$(pipeon_stack_host_mcp_api_key_file)"
  pid_file="$(pipeon_stack_host_mcp_pid_file)"
  port="$(pipeon_stack_host_mcp_port)"
  log_file="$(pipeon_stack_state_dir)/host-mcp.log"

  if pipeon_host_mcp_pid_running && wait_for_host_mcp_ready 4 && pipeon_host_mcp_has_required_tools; then
    printf '[pipeon-dev-stack] reusing host MCP bridge at %s\n' "$(pipeon_stack_host_mcp_url)" >&2
    return 0
  fi

  stop_pipeon_host_mcp_bridge
  printf '[pipeon-dev-stack] starting host MCP bridge at %s\n' "$(pipeon_stack_host_mcp_url)" >&2
  DOCKPIPE_BIN="$DOCKPIPE_BIN" \
  DORKPIPE_BIN="$DORKPIPE_BIN" \
  DOCKPIPE_WORKDIR="$WORKDIR" \
  DOCKPIPE_MCP_TIER=exec \
  DOCKPIPE_MCP_ALLOWED_TOOLS= \
  DOCKPIPE_MCP_IGNORE_ALLOWED_TOOLS=1 \
  DOCKPIPE_MCP_REQUIRE_ABSOLUTE_BIN=1 \
  DOCKPIPE_MCP_RESTRICT_WORKDIR=1 \
  MCP_HTTP_API_KEY_FILE="$api_key_file" \
  MCP_HTTP_INSECURE_LOOPBACK=1 \
    "$MCPD_BIN" \
      -http "127.0.0.1:${port}" \
      -api-key-file "$api_key_file" \
      -insecure-loopback \
      -mcp-tier exec > "$log_file" 2>&1 &
  printf '%s\n' "$!" > "$pid_file"

  if ! wait_for_host_mcp_ready 60; then
    echo "pipeon-dev-stack: host MCP bridge did not become reachable at $(pipeon_stack_host_mcp_url)" >&2
    cat "$log_file" >&2 2>/dev/null || true
    stop_pipeon_host_mcp_bridge
    return 1
  fi
}

warm_pipeon_provider_pools() {
  local status_file warm_output warm_status
  status_file="$(pipeon_stack_state_dir)/provider-pools-status.json"

  warm_output="$("$DORKPIPE_BIN" provider-pool warm --workdir "$WORKDIR" 2>&1)" || warm_status=$?
  warm_status="${warm_status:-0}"
  if [[ "$warm_status" -ne 0 ]]; then
    printf '[pipeon-dev-stack] provider-pool warm failed (exit %s)\n%s\n' "$warm_status" "$warm_output" >&2
  elif [[ -n "$warm_output" ]]; then
    printf '[pipeon-dev-stack] provider pools\n%s\n' "$warm_output" >&2
  fi

  if ! "$DORKPIPE_BIN" provider-pool status --workdir "$WORKDIR" --json > "$status_file"; then
    printf '[pipeon-dev-stack] provider-pool status snapshot failed: %s\n' "$status_file" >&2
  fi
}

stop_pipeon_provider_pools() {
  if [[ -z "${DORKPIPE_BIN:-}" || ! -x "$DORKPIPE_BIN" ]]; then
    return 0
  fi
  "$DORKPIPE_BIN" provider-pool stop --workdir "$WORKDIR" >/dev/null 2>&1 || \
    printf '[pipeon-dev-stack] provider-pool stop failed; continuing stack teardown\n' >&2
}

if ! docker version >/dev/null 2>&1; then
  echo "pipeon-dev-stack: Docker is not reachable" >&2
  exit 1
fi

DOCKPIPE_BIN="$(resolve_repo_tool_bin "${DOCKPIPE_BIN:-}" dockpipe \
  "$WORKDIR/src/bin/dockpipe" \
  "$PROJECT_DIR/src/bin/dockpipe")"
DORKPIPE_BIN="$(resolve_repo_tool_bin "${DORKPIPE_BIN:-}" dorkpipe \
  "$WORKDIR/bin/.dockpipe/tooling/bin/dorkpipe" \
  "$PROJECT_DIR/bin/.dockpipe/tooling/bin/dorkpipe" \
  "$WORKDIR/packages/dorkpipe/bin/dorkpipe" \
  "$PROJECT_DIR/packages/dorkpipe/bin/dorkpipe")"
MCPD_BIN="$(resolve_repo_tool_bin "${MCPD_BIN:-}" mcpd \
  "$WORKDIR/packages/dorkpipe/bin/mcpd" \
  "$PROJECT_DIR/packages/dorkpipe/bin/mcpd" \
  "$WORKDIR/bin/.dockpipe/tooling/bin/mcpd" \
  "$PROJECT_DIR/bin/.dockpipe/tooling/bin/mcpd")"
PIPEON_BIN="$(resolve_pipeon_bin "${PIPEON_BIN:-}")"

ensure_pipeon_stack_state_dir
prepare_pipeon_stack_context
ensure_pipeon_stack_api_key
ensure_pipeon_stack_host_mcp_api_key
ensure_pipeon_stack_mcp_tls_material
write_pipeon_stack_runtime_env

cleanup() {
  if [[ "$AUTODOWN" == "1" ]]; then
    stop_pipeon_provider_pools
    docker rm -f "$CODE_SERVER_CONTAINER_NAME" >/dev/null 2>&1 || true
    docker rm -f "$LEGACY_CODE_SERVER_CONTAINER_NAME" >/dev/null 2>&1 || true
    docker ps -aq --filter "label=com.dockpipe.stack.project=$COMPOSE_PROJECT" | xargs -r docker rm -f >/dev/null 2>&1 || true
    docker ps -aq --filter "label=com.dockpipe.stack.project=$LEGACY_COMPOSE_PROJECT" | xargs -r docker rm -f >/dev/null 2>&1 || true
  fi
  if [[ "$AUTODOWN" == "1" ]]; then
    compose_cmd down >/dev/null 2>&1 || true
    docker compose --env-file "$RUNTIME_ENV" -p "$LEGACY_COMPOSE_PROJECT" -f "$COMPOSE_FILE" --project-directory "$PROJECT_DIR" down >/dev/null 2>&1 || true
  fi
  if [[ "$AUTODOWN" == "1" ]]; then
    stop_pipeon_host_mcp_bridge
  fi
}
trap cleanup EXIT INT TERM

configure_pipeon_stack_gpu

case "${PIPEON_DEV_STACK_PROMPT_RESULT:-}" in
  gpu-setup)
    echo "pipeon-dev-stack: launch paused before starting services so Docker GPU access can be enabled" >&2
    exit 0
    ;;
  cancelled)
    echo "pipeon-dev-stack: launch cancelled before starting services" >&2
    exit 0
    ;;
esac

case "$BUILD_MODE" in
  always)
    printf '[pipeon-dev-stack] PIPEON_DEV_STACK_BUILD=always — force-refresh the Pipeon-managed code-server surface before launch.\n' >&2
    ;;
  auto)
    :
    ;;
  never)
    ;;
  *)
    echo "pipeon-dev-stack: PIPEON_DEV_STACK_BUILD must be auto, always, or never (got $BUILD_MODE)" >&2
    exit 1
    ;;
esac

if ! ensure_pipeon_code_server_surface; then
  exit 1
fi

if ! ensure_executable_binary "$DOCKPIPE_BIN" || ! ensure_executable_binary "$DORKPIPE_BIN" || ! ensure_executable_binary "$MCPD_BIN"; then
  echo "pipeon-dev-stack: required binaries are missing after build step" >&2
  echo "  dockpipe: ${DOCKPIPE_BIN:-<unset>}" >&2
  echo "  dorkpipe: ${DORKPIPE_BIN:-<unset>} (set DORKPIPE_BIN or add dorkpipe to PATH)" >&2
  echo "  mcpd:     ${MCPD_BIN:-<unset>} (set MCPD_BIN or add mcpd to PATH)" >&2
  exit 1
fi

if [[ ! -f "$PIPEON_DESKTOP_SCRIPT" ]]; then
  echo "pipeon-dev-stack: missing desktop launcher script at $PIPEON_DESKTOP_SCRIPT" >&2
  exit 1
fi

compose_up_args=(up -d --remove-orphans)
if [[ "$PIPEON_DEV_STACK_REBUILD_STACK_IMAGE" == "1" ]]; then
  compose_up_args+=(--build)
else
  printf '[pipeon-dev-stack] reusing existing dockpipe-dorkpipe-stack image and compose containers where possible\n' >&2
fi

if ! retry_with_backoff \
  "docker compose up" \
  "${PIPEON_DEV_STACK_COMPOSE_UP_ATTEMPTS:-3}" \
  "${PIPEON_DEV_STACK_COMPOSE_UP_RETRY_DELAY:-5}" \
  compose_cmd "${compose_up_args[@]}"; then
  exit 1
fi

if [[ "$PIPEON_DEV_STACK_REBUILD_STACK_IMAGE" == "1" && -n "$PIPEON_DEV_STACK_STACK_IMAGE_SIGNATURE" ]]; then
  printf '%s\n' "$PIPEON_DEV_STACK_STACK_IMAGE_SIGNATURE" > "$(pipeon_stack_dorkpipe_stack_image_stamp_file)"
fi

if ! verify_pipeon_stack_ollama_gpu; then
  compose_cmd logs ollama >&2 || true
  exit 1
fi

if ! wait_for_mcp_ready 40; then
  echo "pipeon-dev-stack: isolated DorkPipe MCP boundary did not become reachable at $MCP_URL" >&2
  compose_cmd logs dorkpipe-stack pipeon-mcp-proxy >&2 || true
  exit 1
fi

if ! start_pipeon_host_mcp_bridge; then
  exit 1
fi

export CODE_SERVER_WAIT="${CODE_SERVER_WAIT:-1}"
export CODE_SERVER_AUTH="${CODE_SERVER_AUTH:-none}"
export CODE_SERVER_CONTAINER_NAME="$CODE_SERVER_CONTAINER_NAME"
export DORKPIPE_DEV_STACK_PROJECT="$COMPOSE_PROJECT"
export CODE_SERVER_URL="$CODE_SERVER_URL"
export PIPEON_WINDOW_TITLE="${PIPEON_WINDOW_TITLE:-Pipeon}"
export DOCKPIPE_PIPEON="${DOCKPIPE_PIPEON:-1}"
export DOCKPIPE_PIPEON_ALLOW_PRERELEASE="${DOCKPIPE_PIPEON_ALLOW_PRERELEASE:-1}"
export PIPEON_OLLAMA_MODEL="${PIPEON_OLLAMA_MODEL:-$MODEL_NAME}"
export MCP_HTTP_URL="$MCP_URL"
export MCP_HTTP_CONTAINER_URL="$(pipeon_stack_mcp_container_url)"
export PIPEON_HOST_MCP_URL="$(pipeon_stack_host_mcp_url)"
export PIPEON_HOST_MCP_CONTAINER_URL="$(pipeon_stack_host_mcp_container_url)"
export PIPEON_HOST_MCP_API_KEY="$(tr -d ' \t\r\n' < "$(pipeon_stack_host_mcp_api_key_file)")"
export PIPEON_DESKTOP_BIN

if [[ "${CODE_SERVER_IMAGE:-dockpipe-code-server:latest}" == "dockpipe-code-server:latest" ]] \
  && ! docker image inspect dockpipe-code-server:latest >/dev/null 2>&1; then
  export CODE_SERVER_IMAGE="codercom/code-server:latest"
  printf '[pipeon-dev-stack] dockpipe-code-server:latest was not available; falling back to %s (Pipeon branding/extension may be reduced)\n' "$CODE_SERVER_IMAGE" >&2
fi

if [[ "${PIPEON_DEV_STACK_PULL_MODEL:-1}" == "1" ]]; then
  printf '[pipeon-dev-stack] ensuring Ollama model %s is available...\n' "$MODEL_NAME" >&2
  if ! wait_for_ollama_ready 90; then
    echo "pipeon-dev-stack: Ollama did not become ready in time" >&2
    exit 1
  fi
  if ollama_model_available "$MODEL_NAME"; then
    printf '[pipeon-dev-stack] Ollama model %s is already present; skipping pull\n' "$MODEL_NAME" >&2
  else
    retry_with_backoff \
      "ollama model pull ($MODEL_NAME)" \
      "${PIPEON_DEV_STACK_OLLAMA_PULL_ATTEMPTS:-3}" \
      "${PIPEON_DEV_STACK_OLLAMA_PULL_RETRY_DELAY:-5}" \
      compose_cmd exec -T ollama ollama pull "$MODEL_NAME" >&2
  fi
fi

if [[ "${PIPEON_DEV_STACK_PIPEON_BUNDLE:-1}" == "1" && -x "$PIPEON_BIN" ]]; then
  if ! (
    cd "$WORKDIR"
    export DOCKPIPE_BIN
    export DOCKPIPE_WORKDIR="$WORKDIR"
    "$PIPEON_BIN" bundle
  ) >&2; then
    echo "pipeon-dev-stack: pipeon bundle failed using $PIPEON_BIN" >&2
    exit 1
  fi
fi

warm_pipeon_provider_pools

cat >&2 <<EOF
[dockpipe-ready] pipeon-dev-stack
[pipeon-dev-stack] ready
  workdir:      $WORKDIR
  ide:          Pipeon
  ui:           $CODE_SERVER_URL
  mcp:          $MCP_URL
  mcp api key:  $(pipeon_stack_api_key_file)
  host mcp:     $(pipeon_stack_host_mcp_url)
  provider pools: $(pipeon_stack_state_dir)/provider-pools-status.json
  dorkpipe:     isolated compose service
  state:        $(pipeon_stack_state_dir)
  control:      isolated compose service dorkpipe-stack
  ollama gpu:   $(pipeon_stack_gpu_mode)
  gpu status:   $(pipeon_stack_gpu_status)
EOF

desktop_status=0
"$DOCKPIPE_HOST_BASH_BIN" "$PIPEON_DESKTOP_SCRIPT" || desktop_status=$?
if [[ "$desktop_status" -ne 0 ]]; then
  AUTODOWN=0
  printf '[pipeon-dev-stack] Pipeon desktop shell failed to launch (exit %s)\n' "$desktop_status" >&2
  printf '[pipeon-dev-stack] the stack is still running; open Pipeon manually at %s\n' "$CODE_SERVER_URL" >&2
  exit 0
fi
