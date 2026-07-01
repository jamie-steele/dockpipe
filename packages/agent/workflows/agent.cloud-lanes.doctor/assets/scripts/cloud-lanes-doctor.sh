#!/usr/bin/env bash
set -euo pipefail

mkdir -p "$(dockpipe scope artifacts providers)"

doctor_bool() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|on|ON) printf 'true\n' ;;
    *) printf 'false\n' ;;
  esac
}

provider_auth_dir() {
  dockpipe scope resolver "$1" auth-dir
}

provider_container_auth_dir() {
  dockpipe scope resolver "$1" container-auth-dir
}

provider_host_config_file() {
  dockpipe scope resolver "$1" config-file
}

provider_container_config_file() {
  dockpipe scope resolver "$1" container-config-file
}

provider_cli() {
  case "$1" in
    codex) printf 'codex\n' ;;
    claude) printf 'claude\n' ;;
    *) return 1 ;;
  esac
}

provider_live_command() {
  case "$1" in
    codex)
      printf 'codex exec --dangerously-bypass-approvals-and-sandbox %q\n' \
        "Reply exactly: DORKPIPE_AGENT_OK"
      ;;
    claude)
      printf 'claude --dangerously-skip-permissions -p %q\n' \
        "Reply exactly: DORKPIPE_AGENT_OK"
      ;;
    *)
      return 1
      ;;
  esac
}

doctor_providers() {
  local configured="${DORKPIPE_AGENT_DOCTOR_PROVIDERS:-codex claude}"
  local provider
  for provider in ${configured//,/ }; do
    case "$provider" in
      codex|claude) printf '%s\n' "$provider" ;;
      "") ;;
      *) echo "agent.cloud-lanes.doctor: unknown provider ${provider}" >&2; return 1 ;;
    esac
  done
}

doctor_windows_host_path() {
  local path="${1:-}"
  if [[ "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" == Windows_NT:* || "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" == *:msys*:* || "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" == *:cygwin*:* || "${OS:-}:${OSTYPE:-}:${MSYSTEM:-}" == *:*:MINGW* ]]; then
    if [[ "${path}" =~ ^/([A-Za-z])/(.*)$ ]]; then
      local drive rest
      drive="$(printf '%s' "${BASH_REMATCH[1]}" | tr '[:lower:]' '[:upper:]')"
      rest="${BASH_REMATCH[2]//\//\\}"
      printf '%s:\\%s\n' "${drive}" "${rest}"
      return 0
    fi
    if [[ "${path}" =~ ^([A-Za-z]):/ ]]; then
      printf '%s\n' "${path//\//\\}"
      return 0
    fi
  fi
  printf '%s\n' "${path}"
}

run_provider() {
  local provider="$1"
  local provider_dir
  provider_dir="$(dockpipe scope artifacts providers "$provider")"
  local stdout_file="$provider_dir/stdout.txt"
  local stderr_file="$provider_dir/stderr.txt"
  local result_file="$provider_dir/result.json"
  local host_auth container_auth host_config container_config cli live timeout_s mount_mode live_cmd
  local host_auth_exists="false"
  local host_config_exists="false"
  local exit_code=0

  mkdir -p "$provider_dir"
  local source_workdir
  source_workdir="$(doctor_windows_host_path "$(dockpipe scope source)")"
  host_auth="$(doctor_windows_host_path "$(provider_auth_dir "$provider")")"
  container_auth="$(provider_container_auth_dir "$provider")"
  host_config="$(doctor_windows_host_path "$(provider_host_config_file "$provider")")"
  container_config="$(provider_container_config_file "$provider")"
  cli="$(provider_cli "$provider")"
  live="$(doctor_bool "${DORKPIPE_AGENT_DOCTOR_LIVE:-true}")"
  timeout_s="${DORKPIPE_AGENT_DOCTOR_TIMEOUT_SECONDS:-90}"
  mount_mode="rw"
  [[ -d "$host_auth" ]] && host_auth_exists="true"
  [[ -n "$host_config" && -f "$host_config" ]] && host_config_exists="true"

  local args=(
    "--workdir" "$source_workdir"
    "--runtime" "dockerimage"
    "--resolver" "$provider"
    "--no-data"
    "--env" "HOME=/home/node"
    "--env" "PATH=/usr/local/bin:/usr/bin:/bin:/usr/local/games:/usr/games"
    "--env" "DOCKPIPE_RESOLVER_NAME=$provider"
    "--env" "DOCKPIPE_RESOLVER_CLI=$cli"
    "--env" "DOCKPIPE_RESOLVER_AUTH_DIR=$container_auth"
    "--env" "DOCKPIPE_RESOLVER_CONFIG_FILE=$container_config"
    "--env" "DORKPIPE_AGENT_DOCTOR_LIVE=$live"
    "--env" "DORKPIPE_AGENT_DOCTOR_TIMEOUT_SECONDS=$timeout_s"
  )
  if [[ "$host_auth_exists" == "true" ]]; then
    args+=("--mount" "$host_auth:$container_auth:$mount_mode")
  fi
  if [[ "$host_config_exists" == "true" ]]; then
    args+=("--mount" "$host_config:$container_config:$mount_mode")
  fi

  live_cmd="$(provider_live_command "$provider")"
  args+=("--env" "DORKPIPE_AGENT_DOCTOR_LIVE_CMD=$live_cmd")

  set +e
  MSYS2_ARG_CONV_EXCL='*' dockpipe "${args[@]}" -- bash -lc "$(cat <<'SH'
set -u
provider="${DOCKPIPE_RESOLVER_NAME:?provider}"
cli="${DOCKPIPE_RESOLVER_CLI:?cli}"
auth_dir="${DOCKPIPE_RESOLVER_AUTH_DIR:?auth dir}"
config_file="${DOCKPIPE_RESOLVER_CONFIG_FILE:-}"
live="${DORKPIPE_AGENT_DOCTOR_LIVE:-false}"
timeout_s="${DORKPIPE_AGENT_DOCTOR_TIMEOUT_SECONDS:-90}"
skills_dir="${auth_dir}/skills"

if [[ "${provider}" == "claude" && -n "${config_file}" && ! -f "${config_file}" && -d "${auth_dir}/backups" ]]; then
  latest="$(find "${auth_dir}/backups" -maxdepth 1 -type f -name ".claude.json.backup.*" -printf "%T@ %p\n" 2>/dev/null | sort -nr | head -1 | cut -d" " -f2-)"
  if [[ -n "${latest:-}" ]]; then
    cp "${latest}" "${config_file}"
  fi
fi

echo "provider=${provider}"
echo "cli=${cli}"
if command -v "${cli}" >/dev/null 2>&1; then
  echo "cli_present=true"
  echo "cli_path=$(command -v "${cli}")"
  "${cli}" --version 2>&1 | sed 's/^/cli_version: /' || true
else
  echo "cli_present=false"
fi

if [[ -d "${auth_dir}" ]]; then
  echo "auth_dir_present=true"
  echo "auth_dir=${auth_dir}"
else
  echo "auth_dir_present=false"
  echo "auth_dir=${auth_dir}"
fi

if [[ -d "${skills_dir}" ]]; then
  echo "skills_dir_present=true"
  find "${skills_dir}" -mindepth 1 -maxdepth 2 -type f 2>/dev/null | sed 's#^#skill_file: #' | head -50
else
  echo "skills_dir_present=false"
fi

if [[ "${provider}" == "claude" && -n "${config_file}" ]]; then
  if [[ -f "${config_file}" ]]; then
    echo "config_file_present=true"
    echo "config_file=${config_file}"
  else
    echo "config_file_present=false"
    echo "config_file=${config_file}"
  fi
fi

if [[ "${live}" == "true" ]]; then
  echo "live_prompt=true"
  timeout "${timeout_s}" bash -lc "${DORKPIPE_AGENT_DOCTOR_LIVE_CMD}" 2>&1 | sed 's/^/live_output: /'
  live_rc="${PIPESTATUS[0]}"
  echo "live_exit_code=${live_rc}"
  exit "${live_rc}"
else
  echo "live_prompt=false"
fi
SH
)" >"$stdout_file" 2>"$stderr_file"
  exit_code=$?
  set -e

  python3 - "$provider" "$host_auth" "$host_auth_exists" "$host_config" "$host_config_exists" "$container_auth" "$container_config" "$stdout_file" "$stderr_file" "$exit_code" "$live" "$result_file" <<'PY'
import json
import pathlib
import re
import sys

provider, host_auth, host_auth_exists, host_config, host_config_exists, container_auth, container_config, stdout_path, stderr_path, exit_code, live, result_path = sys.argv[1:]
stdout = pathlib.Path(stdout_path).read_text(encoding="utf-8", errors="replace")
stderr = pathlib.Path(stderr_path).read_text(encoding="utf-8", errors="replace")

def has_line(name, value="true"):
    return re.search(rf"^{re.escape(name)}={re.escape(value)}$", stdout, re.MULTILINE) is not None

skill_files = re.findall(r"^skill_file: (.+)$", stdout, re.MULTILINE)
live_outputs = re.findall(r"^live_output: (.+)$", stdout, re.MULTILINE)
live_text = "\n".join(live_outputs).strip()
live_ok = live != "true" or "DORKPIPE_AGENT_OK" in live_text
checks = {
    "host_auth_dir_present": host_auth_exists == "true",
    "host_config_file_present": host_config_exists == "true",
    "container_cli_present": has_line("cli_present"),
    "container_auth_dir_present": has_line("auth_dir_present"),
    "container_config_file_present": provider != "claude" or has_line("config_file_present"),
    "container_skills_dir_present": has_line("skills_dir_present"),
    "live_prompt_requested": live == "true",
    "live_prompt_ok": live_ok,
}
status = "pass" if all(v for k, v in checks.items() if k not in {"live_prompt_requested", "host_config_file_present"}) and int(exit_code) == 0 else "fail"
issues = []
if not checks["host_auth_dir_present"]:
    issues.append(f"host auth directory is missing: {host_auth}")
if not checks["container_cli_present"]:
    issues.append(f"{provider} CLI is not present in the resolver container")
if not checks["container_auth_dir_present"]:
    issues.append(f"auth directory was not visible inside container: {container_auth}")
if provider == "claude" and not checks["container_config_file_present"]:
    issues.append(f"Claude config file was not visible or restored inside container: {container_config}")
if not checks["container_skills_dir_present"]:
    issues.append(f"skills directory was not visible inside container: {container_auth}/skills")
if live == "true" and not live_ok:
    if "Not logged in" in live_text or "Please run /login" in live_text:
        issues.append(f"{provider} CLI is not logged in inside the resolver container; run {provider} /login on the host and rerun this doctor")
    else:
        issues.append("tiny live prompt did not return expected marker")
if int(exit_code) != 0:
    issues.append(f"container command exited {exit_code}")

pathlib.Path(result_path).write_text(json.dumps({
    "provider": provider,
    "status": status,
    "exit_code": int(exit_code),
    "host_auth_dir": host_auth,
    "host_config_file": host_config,
    "container_auth_dir": container_auth,
    "container_config_file": container_config,
    "checks": checks,
    "skill_file_count": len(skill_files),
    "skill_files_sample": skill_files[:20],
    "live_output_preview": live_text[:500],
    "issues": issues,
    "stdout": str(stdout_path),
    "stderr": str(stderr_path),
}, indent=2) + "\n", encoding="utf-8")
PY
}

mapfile -t providers < <(doctor_providers)
if [[ "${#providers[@]}" -eq 0 ]]; then
  echo "agent.cloud-lanes.doctor: no providers selected" >&2
  exit 1
fi
for provider in "${providers[@]}"; do
  echo "[agent.cloud-lanes.doctor] checking ${provider}"
  run_provider "$provider"
done

python3 - "$(dockpipe scope artifacts)" "${providers[@]}" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
providers = sys.argv[2:]
results = []
for provider in providers:
    path = root / "providers" / provider / "result.json"
    results.append(json.loads(path.read_text(encoding="utf-8")))
status = "pass" if all(r["status"] == "pass" for r in results) else "fail"
summary = {
    "status": status,
    "providers": results,
}
(root / "result.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
print(f"agent.cloud-lanes.doctor: {status}")
for r in results:
    print(f"- {r['provider']}: {r['status']} ({', '.join(r['issues']) if r['issues'] else 'ok'})")
raise SystemExit(0 if status == "pass" else 1)
PY
