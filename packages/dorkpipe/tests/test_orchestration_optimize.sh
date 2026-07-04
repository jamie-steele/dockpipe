#!/usr/bin/env bash
set -euo pipefail
trap 'rc=$?; echo "test_orchestration_optimize failed at line ${LINENO}: ${BASH_COMMAND}" >&2; if [[ -n "${tmp:-}" && -f "$tmp/optimize.err" ]]; then cat "$tmp/optimize.err" >&2; fi; if [[ -n "${tmp:-}" && -f "$tmp/optimize-iterations.err" ]]; then cat "$tmp/optimize-iterations.err" >&2; fi; exit "$rc"' ERR

ROOT="$(git rev-parse --show-toplevel)"
SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"

mkdir -p "$ROOT/bin/.dockpipe/tmp/package-tests"
tmp="$(mktemp -d "$ROOT/bin/.dockpipe/tmp/package-tests/orchestration-optimize.XXXXXX")"
if command -v cygpath >/dev/null 2>&1; then
  tmp_unix="$(cygpath -u "$tmp")"
  tmp_win="$(cygpath -m "$tmp")"
else
  tmp_unix="$tmp"
  tmp_win="$tmp"
fi
trap 'rm -rf "$tmp"' EXIT
fake_dockpipe="$tmp/dockpipe"
fake_helper="$tmp/orchestrate-helper"
operation_log="$tmp/operation.log"
workflow_log="$tmp/workflows.log"

cat >"$fake_dockpipe" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  get)
    if [[ "${2:-}" == "script_dir" ]]; then
      printf '%s\n' "${FAKE_SCRIPT_DIR:?}"
      exit 0
    fi
    ;;
  sdk)
    cat <<'SDK'
dockpipe_sdk() {
  case "${1:-}" in
    init-script)
      SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:?}"
      ROOT="${DOCKPIPE_WORKDIR:?}"
      export SCRIPT_DIR ROOT
      ;;
    get)
      case "${2:-}" in
        workdir) printf '%s\n' "${DOCKPIPE_WORKDIR:?}" ;;
        script_dir) printf '%s\n' "${DOCKPIPE_SCRIPT_DIR:?}" ;;
        *) return 1 ;;
      esac
      ;;
    require)
      if [[ "${2:-}" == "dockpipe-bin" ]]; then
        printf '%s\n' "${DOCKPIPE_BIN:?}"
        return 0
      fi
      return 1
      ;;
    *)
      return 1
      ;;
  esac
}
SDK
    exit 0
    ;;
  scope)
    if [[ "${2:-}" == "--workdir" ]]; then
      set -- "$1" "${@:4}"
    fi
    if [[ "${2:-}" == "artifacts" ]]; then
      shift 2
      printf '%s\n' "${FAKE_ARTIFACT_ROOT:?}${1:+/$*}"
      exit 0
    fi
    if [[ "${2:-}" == "--package" ]]; then
      package="${3:-}"
      shift 3
      printf '%s\n' "${FAKE_PACKAGE_ROOT:?}/${package}${1:+/$*}"
      exit 0
    fi
    if [[ "${2:-}" == "workflow" ]]; then
      workflow="${3:-}"
      suffix="${4:-}"
      case "${suffix}" in
        orchestrate) printf '%s\n' "${FAKE_TARGET_ROOT:?}/${workflow}/orchestrate" ;;
        optimize)
          if [[ "${5:-}" == "iterations" ]]; then
            printf '%s\n' "${FAKE_OPTIMIZER_ROOT:?}/${workflow}/optimize/iterations"
          else
            printf '%s\n' "${FAKE_OPTIMIZER_ROOT:?}/${workflow}/optimize"
          fi
          ;;
        *) return 1 ;;
      esac
      exit 0
    fi
    ;;
  result)
    shift
    unit=""
    status=""
    duration_ms=""
    fields=()
    while (($#)); do
      case "${1:-}" in
        --unit) unit="${2:-}"; shift 2 ;;
        --status) status="${2:-}"; shift 2 ;;
        --duration-ms) duration_ms="${2:-}"; shift 2 ;;
        --id) fields+=("${2:-}"); shift 2 ;;
        --error) fields+=("error=${2:-}"); shift 2 ;;
        *) shift ;;
      esac
    done
    {
      printf 'unit=%s status=%s' "${unit}" "${status}"
      if [[ -n "${duration_ms}" && "${status}" != "start" ]]; then
        printf ' duration_ms=%s' "${duration_ms}"
      fi
      for field in "${fields[@]}"; do
        printf ' %s' "${field}"
      done
      printf '\n'
    } >> "${FAKE_OPERATION_LOG:?}"
    exit 0
    ;;
esac
if [[ " $* " == *" --workflow "* ]]; then
  printf '%s\n' "$*" >> "${FAKE_WORKFLOW_LOG:?}"
  mkdir -p "${FAKE_OPTIMIZER_ROOT:?}/${DORKPIPE_OPTIMIZER_TARGET_WORKFLOW:?}/optimize/apply-if-enabled"
  mkdir -p "${FAKE_OPTIMIZER_ROOT:?}/${DORKPIPE_OPTIMIZER_TARGET_WORKFLOW:?}/optimize/propose"
  cat > "${FAKE_OPTIMIZER_ROOT:?}/${DORKPIPE_OPTIMIZER_TARGET_WORKFLOW:?}/optimize/apply-if-enabled/result.json" <<EOF
{"status":"ready"}
EOF
  cat > "${FAKE_OPTIMIZER_ROOT:?}/${DORKPIPE_OPTIMIZER_TARGET_WORKFLOW:?}/optimize/propose/result.json" <<EOF
{"status":"ready","invalid_patch":false}
EOF
  exit 0
fi
echo "fake dockpipe: unsupported args: $*" >&2
exit 1
SH
chmod +x "$fake_dockpipe"

cat >"$fake_helper" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  optimizer-result-status)
    file="${2:?result json}"
    [[ -f "$file" ]] || exit 0
    sed -n 's/.*"status"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$file" | head -1
    ;;
  optimizer-propose-invalid)
    file="${2:?result json}"
    if [[ -f "$file" ]] && grep -Eq '"invalid_patch"[[:space:]]*:[[:space:]]*true' "$file"; then
      printf 'true\n'
    else
      printf 'false\n'
    fi
    ;;
  *)
    echo "fake orchestrate-helper: unsupported args: $*" >&2
    exit 1
    ;;
esac
SH
chmod +x "$fake_helper"

export PATH="$tmp_unix:$PATH"
hash -r
export DOCKPIPE_BIN="$fake_dockpipe"
export DOCKPIPE_WORKDIR="$tmp"
export DOCKPIPE_SCRIPT_DIR="$SCRIPT_DIR"
export DOCKPIPE_ASSETS_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets"
export DOCKPIPE_WORKFLOW_NAME="docs.optimize-orchestrate.test"
export DORKPIPE_OPTIMIZER_ACTION="iterate"
export DORKPIPE_OPTIMIZER_ITERATIONS="1"
export DORKPIPE_OPTIMIZER_TARGET_WORKFLOW="docs.orchestrate.test"
export FAKE_SCRIPT_DIR="$SCRIPT_DIR"
export FAKE_TARGET_ROOT="$tmp/target"
export FAKE_OPTIMIZER_ROOT="${tmp_win}/optimizer"
export FAKE_ARTIFACT_ROOT="$tmp/artifacts"
export FAKE_PACKAGE_ROOT="$tmp/package"
export FAKE_OPERATION_LOG="$operation_log"
export FAKE_WORKFLOW_LOG="$workflow_log"
export DORKPIPE_ORCH_HELPER_BIN="$fake_helper"

bash "$SCRIPT_DIR/orchestrate-optimize.sh" 2>"$tmp/optimize.err"

grep -Fq -- "unit=orchestrate.optimize status=start" "$operation_log"
grep -Fq -- "action=iterate" "$operation_log"
grep -Fq -- "unit=orchestrate.optimize status=done" "$operation_log"
grep -Fq -- "result_status=skipped" "$operation_log"
grep -Fq -- "reason=single_optimizer_pass" "$operation_log"

result_path="${tmp_win}/optimizer/docs.orchestrate.test/optimize/iterate/result.json"
if [[ ! -f "$result_path" ]]; then
  echo "expected optimizer result at $result_path" >&2
  find "$tmp" -type f >&2
  exit 1
fi
if grep -Fq -- "${tmp}/${tmp_win}" "$operation_log"; then
  echo "optimizer Windows absolute path was incorrectly prefixed by ROOT" >&2
  exit 1
fi

rm -f "$operation_log" "$workflow_log"
export DORKPIPE_OPTIMIZER_ITERATIONS="2"
bash "$SCRIPT_DIR/orchestrate-optimize.sh" 2>"$tmp/optimize-iterations.err"

grep -Fq -- "unit=orchestrate.optimize.iteration status=start" "$operation_log"
grep -Fq -- "unit=orchestrate.optimize.iteration status=done" "$operation_log"
grep -Fq -- "iteration=1" "$operation_log"
grep -Fq -- "iterations=2" "$operation_log"
grep -Fq -- "result_status=ready" "$operation_log"
grep -Fq -- "child_workflow=docs.optimize-orchestrate" "$operation_log"
grep -Fq -- "unit=orchestrate.optimize status=done" "$operation_log"
grep -Fq -- "completed_child_iterations=1" "$operation_log"
grep -Fq -- "--workflow docs.optimize-orchestrate" "$workflow_log"

echo "test_orchestration_optimize OK"
