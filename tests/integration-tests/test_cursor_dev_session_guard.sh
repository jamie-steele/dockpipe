#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "$(dirname "${BASH_SOURCE[0]}")/helpers.sh"
require_cursor_dev_script "$REPO_ROOT"
SCRIPT="$REPLY"

test_existing_session_guard() {
  local tmp fakebin workdir docker_log pid out rc
  tmp="$(mktemp -d)"
  fakebin="${tmp}/bin"
  workdir="${tmp}/work"
  docker_log="${tmp}/docker.log"
  mkdir -p "${fakebin}" "${workdir}/bin/.dockpipe/packages/cursor-dev"

  cat >"${fakebin}/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log_file="${FAKE_DOCKER_LOG:?}"
printf '%s\n' "$*" >>"$log_file"
case "${1:-}" in
  info)
    exit 0
    ;;
  image)
    if [[ "${2:-}" == "inspect" ]]; then
      exit 0
    fi
    ;;
  inspect)
    if [[ "${2:-}" == "-f" ]]; then
      printf 'running\n'
      exit 0
    fi
    ;;
  run)
    echo "unexpected docker run" >&2
    exit 99
    ;;
esac
echo "unexpected docker invocation: $*" >&2
exit 98
EOF
  chmod +x "${fakebin}/docker"

  sleep 60 &
  pid=$!
  cat >"${workdir}/bin/.dockpipe/packages/cursor-dev/active-session.env" <<EOF
CURSOR_DEV_ACTIVE_PID=${pid}
CURSOR_DEV_ACTIVE_CONTAINER=existing-container
CURSOR_DEV_ACTIVE_WORKDIR=${workdir}
EOF

  out="${tmp}/out.log"
  set +e
  PATH="${fakebin}:$PATH" \
    FAKE_DOCKER_LOG="${docker_log}" \
    DOCKPIPE_WORKDIR="${workdir}" \
    bash "${SCRIPT}" >"${out}" 2>&1
  rc=$?
  set -e

  kill "${pid}" 2>/dev/null || true
  wait "${pid}" 2>/dev/null || true

  if [[ ${rc} -ne 0 ]]; then
    echo "test_existing_session_guard FAIL: expected zero exit, got ${rc}"
    cat "${out}"
    return 1
  fi
  if ! grep -q "An active session is already registered for this workdir" "${out}"; then
    echo "test_existing_session_guard FAIL: missing active-session warning"
    cat "${out}"
    return 1
  fi
  if grep -q '^run ' "${docker_log}" || grep -q 'unexpected docker run' "${out}"; then
    echo "test_existing_session_guard FAIL: docker run should not happen when guard triggers"
    cat "${docker_log}"
    cat "${out}"
    return 1
  fi

  rm -rf "${tmp}"
  echo "test_existing_session_guard OK"
}

run_tests() {
  test_existing_session_guard
}

run_tests
