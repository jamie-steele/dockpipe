#!/usr/bin/env bash
set -euo pipefail

report_desktop_failure() {
  local exit_code="$1"
  local line_no="$2"
  local command="$3"
  printf 'pipeon-dev-stack: desktop.sh failed at line %s while running: %s (exit %s)\n' \
    "$line_no" "$command" "$exit_code" >&2
}

trap 'report_desktop_failure "$?" "${LINENO}" "${BASH_COMMAND}"' ERR

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/common.sh"

WORKDIR="$(pipeon_stack_workdir)"
CODE_SERVER_CONTAINER_NAME="${CODE_SERVER_CONTAINER_NAME:-$(pipeon_stack_code_server_name)}"
DOCKER_STACK_PROJECT="${DORKPIPE_DEV_STACK_PROJECT:-$(pipeon_stack_compose_project)}"
DOCKER_NETWORK_NAME="${DORKPIPE_DEV_STACK_NETWORK:-$(pipeon_stack_compose_network)}"
CODE_SERVER_PORT="$(pipeon_stack_code_server_port)"
CODE_SERVER_URL="${CODE_SERVER_URL:-$(pipeon_stack_code_server_url)}"
MCP_HTTP_CONTAINER_URL="${MCP_HTTP_CONTAINER_URL:-$(pipeon_stack_mcp_container_url)}"
CODE_SERVER_HOME="$(pipeon_stack_code_server_home)"
CODE_SERVER_IMAGE="${CODE_SERVER_IMAGE:-dockpipe-code-server:latest}"
CODE_SERVER_AUTH="${CODE_SERVER_AUTH:-none}"
PIPEON_CODE_SERVER_SETTINGS_FILE="${PIPEON_CODE_SERVER_SETTINGS_FILE:-$(pipeon_stack_repo_root)/packages/pipeon/resolvers/pipeon/vscode-extension/code-server-user-settings.json}"
PIPEON_CODE_SERVER_THEME="${PIPEON_CODE_SERVER_THEME:-$(pipeon_stack_host_theme 2>/dev/null || true)}"
PIPEON_DESKTOP_BIN="${PIPEON_DESKTOP_BIN:-$(pipeon_stack_desktop_bin)}"
PIPEON_WINDOW_TITLE="${PIPEON_WINDOW_TITLE:-Pipeon}"
WAIT_FOR_UI="${CODE_SERVER_WAIT:-1}"
PIPEON_DEV_STACK_OPEN="${PIPEON_DEV_STACK_OPEN:-1}"

pipeon_code_server_entrypoint() {
  if pipeon_stack_is_windows_host; then
    # Prevent MSYS/Git Bash from rewriting the Linux container entrypoint into
    # a host Git path like C:/Program Files/Git/usr/bin/bash.
    printf '%s\n' '//bin//bash'
    return 0
  fi
  printf '%s\n' '/bin/bash'
}

pipeon_wait_for_http() {
  local url="$1"
  local attempts="${2:-120}"
  local i
  for ((i = 0; i < attempts; i++)); do
    if curl -fsS -I "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}

pipeon_expected_workspace_entries() {
  local root="$1"
  if [[ ! -d "$root" ]]; then
    return 0
  fi
  (
    cd "$root"
    find . -mindepth 1 -maxdepth 1 -printf '%f\n' 2>/dev/null | sed '/^$/d' | sort | head -20
  )
}

pipeon_validate_code_server_workspace_mount() {
  local expected actual overlap
  expected="$(pipeon_expected_workspace_entries "$WORKDIR")"
  if [[ -z "$expected" ]]; then
    return 0
  fi

  actual="$(
    docker exec "$CODE_SERVER_CONTAINER_NAME" sh -lc '
      if [ ! -d /work ]; then
        exit 2
      fi
      find /work -mindepth 1 -maxdepth 1 -printf "%f\n" 2>/dev/null | sed "/^$/d" | sort | head -20
    ' 2>/dev/null || true
  )"
  if [[ -z "$actual" ]]; then
    echo "pipeon-dev-stack: code-server workspace mount is empty or unreadable at /work" >&2
    return 1
  fi

  overlap="$(
    printf '%s\n' "$expected" | while IFS= read -r host_entry; do
      [[ -n "$host_entry" ]] || continue
      if printf '%s\n' "$actual" | grep -Fqx "$host_entry"; then
        printf '%s\n' "$host_entry"
      fi
    done
  )"
  if [[ -n "$overlap" ]]; then
    return 0
  fi

  echo "pipeon-dev-stack: code-server workspace mount does not match the requested workdir" >&2
  echo "  requested workdir: $WORKDIR" >&2
  echo "  expected top-level entries:" >&2
  printf '%s\n' "$expected" | sed 's/^/    - /' >&2
  echo "  code-server /work entries:" >&2
  printf '%s\n' "$actual" | sed 's/^/    - /' >&2
  return 1
}

pipeon_seed_code_server_settings() {
  local target_dir="$CODE_SERVER_HOME/.local/share/code-server/User"
  local target_path="$target_dir/settings.json"
  local defaults_path="${PIPEON_CODE_SERVER_SETTINGS_FILE:-}"

  mkdir -p "$target_dir"
  if [[ -z "$defaults_path" ]] || [[ ! -f "$defaults_path" ]]; then
    return 0
  fi

  if command -v python3 >/dev/null 2>&1; then
    DEFAULTS_PATH="$defaults_path" TARGET_PATH="$target_path" python3 - <<'PY'
import json
import os
from pathlib import Path

defaults_path = Path(os.environ["DEFAULTS_PATH"])
target_path = Path(os.environ["TARGET_PATH"])

defaults = json.loads(defaults_path.read_text(encoding="utf-8"))
existing = {}
if target_path.exists():
    try:
        existing = json.loads(target_path.read_text(encoding="utf-8"))
    except Exception:
        existing = {}

existing.pop("workbench.panel.defaultLocation", None)

merged = dict(existing)
merged.update(defaults)
theme = os.environ.get("PIPEON_CODE_SERVER_THEME", "").strip().lower()
if theme == "dark":
    merged["window.autoDetectColorScheme"] = False
    merged["workbench.colorTheme"] = "Default Dark+"
elif theme == "light":
    merged["window.autoDetectColorScheme"] = False
    merged["workbench.colorTheme"] = "Default Light+"
target_path.write_text(json.dumps(merged, indent=2) + "\n", encoding="utf-8")
PY
    return 0
  fi

  if command -v node >/dev/null 2>&1; then
    DEFAULTS_PATH="$defaults_path" TARGET_PATH="$target_path" node - <<'NODE'
const fs = require('fs');

const defaultsPath = process.env.DEFAULTS_PATH;
const targetPath = process.env.TARGET_PATH;

const defaults = JSON.parse(fs.readFileSync(defaultsPath, 'utf8'));
let existing = {};

if (fs.existsSync(targetPath)) {
  try {
    existing = JSON.parse(fs.readFileSync(targetPath, 'utf8'));
  } catch {
    existing = {};
  }
}

delete existing['workbench.panel.defaultLocation'];

const merged = { ...existing, ...defaults };
const theme = String(process.env.PIPEON_CODE_SERVER_THEME || '').trim().toLowerCase();
if (theme === 'dark') {
  merged['window.autoDetectColorScheme'] = false;
  merged['workbench.colorTheme'] = 'Default Dark+';
} else if (theme === 'light') {
  merged['window.autoDetectColorScheme'] = false;
  merged['workbench.colorTheme'] = 'Default Light+';
}
fs.writeFileSync(targetPath, `${JSON.stringify(merged, null, 2)}\n`, 'utf8');
NODE
    return 0
  fi

  cp "$defaults_path" "$target_path"
}

pipeon_start_code_server() {
  local cid
  local docker_workdir docker_home
  mkdir -p "$CODE_SERVER_HOME"
  pipeon_seed_code_server_settings
  docker_workdir="$(pipeon_stack_docker_host_path "$WORKDIR")"
  docker_home="$(pipeon_stack_docker_host_path "$CODE_SERVER_HOME")"
  if docker ps --format '{{.Names}}' | grep -qx "$CODE_SERVER_CONTAINER_NAME"; then
    return 0
  fi
  if docker ps -a --format '{{.Names}}' | grep -qx "$CODE_SERVER_CONTAINER_NAME"; then
    docker rm -f "$CODE_SERVER_CONTAINER_NAME" >/dev/null 2>&1 || true
  fi

  if pipeon_stack_is_windows_host; then
    local powershell_bin
    powershell_bin="$(pipeon_stack_powershell_bin)" || {
      echo "pipeon-dev-stack: PowerShell is required for the Windows code-server launch path" >&2
      return 1
    }
    cid="$(
      CODE_SERVER_CONTAINER_NAME="$CODE_SERVER_CONTAINER_NAME" \
      DOCKER_STACK_PROJECT="$DOCKER_STACK_PROJECT" \
      DOCKER_NETWORK_NAME="$DOCKER_NETWORK_NAME" \
      CODE_SERVER_PORT="$CODE_SERVER_PORT" \
      CODE_SERVER_AUTH="$CODE_SERVER_AUTH" \
      DOCKPIPE_PIPEON="${DOCKPIPE_PIPEON:-1}" \
      DOCKPIPE_PIPEON_ALLOW_PRERELEASE="${DOCKPIPE_PIPEON_ALLOW_PRERELEASE:-1}" \
      PIPEON_OLLAMA_MODEL="${PIPEON_OLLAMA_MODEL:-llama3.2}" \
      MCP_HTTP_CONTAINER_URL="${MCP_HTTP_CONTAINER_URL:-}" \
      DOCKER_WORKDIR="$docker_workdir" \
      DOCKER_HOME="$docker_home" \
      CODE_SERVER_IMAGE="$CODE_SERVER_IMAGE" \
      "$powershell_bin" -NoProfile -Command '
        $ErrorActionPreference = "Stop"
        $argsList = @(
          "run", "-d",
          "--name", $env:CODE_SERVER_CONTAINER_NAME,
          "--label", "com.dockpipe.stack.project=$($env:DOCKER_STACK_PROJECT)",
          "--label", "com.dockpipe.stack.role=code-server",
          "--network", $env:DOCKER_NETWORK_NAME,
          "--add-host=host.docker.internal:host-gateway",
          "-p", "127.0.0.1:$($env:CODE_SERVER_PORT):8080",
          "-e", "HOME=/home/coder",
          "-e", "XDG_CACHE_HOME=/home/coder/.cache",
          "-e", "XDG_CONFIG_HOME=/home/coder/.config",
          "-e", "XDG_DATA_HOME=/home/coder/.local/share",
          "-e", "DOTNET_CLI_HOME=/home/coder/.dotnet",
          "-e", "GOCACHE=/home/coder/.cache/go-build",
          "-e", "GIT_CONFIG_GLOBAL=/home/coder/.gitconfig",
          "-e", "DOCKPIPE_PIPEON=$($env:DOCKPIPE_PIPEON)",
          "-e", "DOCKPIPE_PIPEON_ALLOW_PRERELEASE=$($env:DOCKPIPE_PIPEON_ALLOW_PRERELEASE)",
          "-e", "DOCKPIPE_WORKDIR=/work",
          "-e", "PIPEON_OLLAMA_MODEL=$($env:PIPEON_OLLAMA_MODEL)",
          "-e", "MCP_HTTP_URL=$($env:MCP_HTTP_CONTAINER_URL)",
          "--mount", "type=bind,src=$($env:DOCKER_WORKDIR),dst=/work",
          "--mount", "type=bind,src=$($env:DOCKER_HOME),dst=/home/coder",
          $env:CODE_SERVER_IMAGE,
          "--bind-addr", "0.0.0.0:8080",
          "--auth", $env:CODE_SERVER_AUTH,
          "--user-data-dir", "/home/coder/.local/share/code-server",
          "--extensions-dir", "/opt/pipeon/extensions",
          "/work"
        )
        $cid = & docker @argsList
        if ($LASTEXITCODE -ne 0) {
          exit $LASTEXITCODE
        }
        Write-Output $cid
      '
    )"
  else
  cid="$(
    MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL='*' docker run -d \
    --name "$CODE_SERVER_CONTAINER_NAME" \
    --label "com.dockpipe.stack.project=$DOCKER_STACK_PROJECT" \
    --label "com.dockpipe.stack.role=code-server" \
    --entrypoint "$(pipeon_code_server_entrypoint)" \
    --network "$DOCKER_NETWORK_NAME" \
    --add-host=host.docker.internal:host-gateway \
    -p "127.0.0.1:${CODE_SERVER_PORT}:8080" \
    -e HOME=/home/coder \
    -e XDG_CACHE_HOME=/home/coder/.cache \
    -e XDG_CONFIG_HOME=/home/coder/.config \
    -e XDG_DATA_HOME=/home/coder/.local/share \
    -e DOTNET_CLI_HOME=/home/coder/.dotnet \
    -e GOCACHE=/home/coder/.cache/go-build \
    -e GIT_CONFIG_GLOBAL=/home/coder/.gitconfig \
    -e DOCKPIPE_PIPEON="${DOCKPIPE_PIPEON:-1}" \
    -e DOCKPIPE_PIPEON_ALLOW_PRERELEASE="${DOCKPIPE_PIPEON_ALLOW_PRERELEASE:-1}" \
    -e DOCKPIPE_WORKDIR=/work \
    -e PIPEON_OLLAMA_MODEL="${PIPEON_OLLAMA_MODEL:-llama3.2}" \
    -e MCP_HTTP_URL="${MCP_HTTP_CONTAINER_URL:-}" \
    --mount "type=bind,src=${docker_workdir},dst=/work" \
    --mount "type=bind,src=${docker_home},dst=/home/coder" \
    "$CODE_SERVER_IMAGE" \
    -lc '
      set -e
      exec code-server \
        --bind-addr 0.0.0.0:8080 \
        --auth "'"$CODE_SERVER_AUTH"'" \
        --user-data-dir /home/coder/.local/share/code-server \
        --extensions-dir /opt/pipeon/extensions \
        /work
    '
  )"
  fi

  sleep 1
  if ! docker ps --format '{{.Names}}' | grep -qx "$CODE_SERVER_CONTAINER_NAME"; then
    echo "pipeon-dev-stack: code-server container exited during startup" >&2
    docker logs "$CODE_SERVER_CONTAINER_NAME" >&2 || true
    return 1
  fi
  if ! pipeon_validate_code_server_workspace_mount; then
    docker logs "$CODE_SERVER_CONTAINER_NAME" >&2 || true
    return 1
  fi
}

if [[ ! -x "$PIPEON_DESKTOP_BIN" ]]; then
  echo "pipeon-dev-stack: Pipeon desktop binary not found at $PIPEON_DESKTOP_BIN" >&2
  echo "Build it with: packages/pipeon/assets/scripts/build.sh desktop" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "pipeon-dev-stack: curl is required to wait for the Pipeon UI" >&2
  exit 1
fi

pipeon_start_code_server

if [[ "$WAIT_FOR_UI" == "1" ]]; then
  if ! pipeon_wait_for_http "$CODE_SERVER_URL" 120; then
    echo "pipeon-dev-stack: Pipeon UI did not become reachable at $CODE_SERVER_URL" >&2
    docker logs "$CODE_SERVER_CONTAINER_NAME" >&2 || true
    exit 1
  fi
fi

if [[ "$PIPEON_DEV_STACK_OPEN" != "1" ]]; then
  printf '[pipeon-dev-stack] desktop auto-open disabled; Pipeon UI remains available at %s\n' "$CODE_SERVER_URL" >&2
  exit 0
fi

if ! pipeon_stack_is_windows_host && [[ -z "${DISPLAY:-}" && -z "${WAYLAND_DISPLAY:-}" ]]; then
  printf '[pipeon-dev-stack] no GUI display detected; Pipeon UI remains available at %s\n' "$CODE_SERVER_URL" >&2
  exit 0
fi

printf '[pipeon-dev-stack] opening Pipeon desktop shell at %s\n' "$CODE_SERVER_URL" >&2
PIPEON_URL="$CODE_SERVER_URL" PIPEON_WINDOW_TITLE="$PIPEON_WINDOW_TITLE" exec "$PIPEON_DESKTOP_BIN"
