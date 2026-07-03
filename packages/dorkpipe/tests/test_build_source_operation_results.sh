#!/usr/bin/env bash
set -euo pipefail
trap 'rc=$?; echo "test_build_source_operation_results failed at line ${LINENO}: ${BASH_COMMAND}" >&2; exit "$rc"' ERR

ROOT="$(git rev-parse --show-toplevel)"
SCRIPT="$ROOT/packages/dorkpipe/assets/scripts/build-source.sh"

mkdir -p "$ROOT/bin/.dockpipe/tmp/package-tests"
tmp="$(mktemp -d "$ROOT/bin/.dockpipe/tmp/package-tests/build-source.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

fake_bin="$tmp/bin"
fake_bin_unix="$(cygpath -u "$fake_bin")"
fake_repo="$tmp/repo"
fake_package_root="$fake_repo/packages/dorkpipe"
mkdir -p "$fake_bin"
mkdir -p "$fake_package_root/lib" "$fake_package_root/mcp"
printf '0.0.0-test\n' > "$fake_repo/VERSION"
operation_log="$tmp/operation.log"
go_log="$tmp/go.log"

cat >"$fake_bin/dockpipe" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" != "result" ]]; then
  echo "fake dockpipe: unsupported args: $*" >&2
  exit 1
fi
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
  printf 'unit=%s status=%s' "$unit" "$status"
  if [[ -n "$duration_ms" && "$status" != "start" ]]; then
    printf ' duration_ms=%s' "$duration_ms"
  fi
  for field in "${fields[@]}"; do
    printf ' %s' "$field"
  done
  printf '\n'
} >> "${FAKE_OPERATION_LOG:?}"
SH
chmod +x "$fake_bin/dockpipe"

cat >"$fake_bin/go" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "env" && "${2:-}" == "GOEXE" ]]; then
  printf '\n'
  exit 0
fi
if [[ "${1:-}" != "build" ]]; then
  echo "fake go: unsupported args: $*" >&2
  exit 1
fi
out=""
args=("$@")
for ((i = 0; i < ${#args[@]}; i++)); do
  if [[ "${args[$i]}" == "-o" ]]; then
    out="${args[$((i + 1))]:-}"
    break
  fi
done
[[ -n "$out" ]] || { echo "fake go: missing -o" >&2; exit 1; }
mkdir -p "$(dirname "$out")"
printf 'fake binary\n' > "$out"
printf '%s\n' "$*" >> "${FAKE_GO_LOG:?}"
SH
chmod +x "$fake_bin/go"

export PATH="$fake_bin_unix:$PATH"
hash -r
export FAKE_OPERATION_LOG="$operation_log"
export FAKE_GO_LOG="$go_log"
export DOCKPIPE_BIN="$fake_bin/dockpipe"
export DOCKPIPE_PACKAGE_ROOT="$fake_package_root"

bash "$SCRIPT"

for tool in dorkpipe mcpd skills-render orchestrate-helper; do
  grep -Fq -- "unit=package.source.tool status=start" "$operation_log"
  grep -Fq -- "unit=package.source.tool status=done" "$operation_log"
  grep -Fq -- "tool=$tool" "$operation_log"
done

[[ "$(grep -c 'unit=package.source.tool status=start' "$operation_log")" -eq 4 ]]
[[ "$(grep -c 'unit=package.source.tool status=done' "$operation_log")" -eq 4 ]]
grep -Fq -- "./cmd/dorkpipe" "$go_log"
grep -Fq -- "./cmd/mcpd" "$go_log"
grep -Fq -- "./cmd/skills-render" "$go_log"
grep -Fq -- "./cmd/orchestrate-helper" "$go_log"

echo "test_build_source_operation_results OK"
