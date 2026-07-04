#!/usr/bin/env bash
# Local CI emulation:
# - Linux host: run the Linux GitHub Actions job through act, then run the Windows subset through
#   the first-party windows-vm workflow.
# - Windows host: run the Windows subset directly on the host, then run the Linux GitHub Actions
#   job through act against the Windows Docker daemon.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

LINUX_JOB="${DOCKPIPE_CI_EMULATE_JOB:-test}"
WINDOWS_JOB_LABEL="${DOCKPIPE_CI_EMULATE_WINDOWS_JOB:-test-windows}"
WINDOWS_WORKFLOW="${DOCKPIPE_CI_EMULATE_WINDOWS_WORKFLOW:-windows-vm}"
EVENT="${DOCKPIPE_CI_EMULATE_EVENT:-pull_request}"
WORKFLOW="${DOCKPIPE_CI_EMULATE_WORKFLOW:-.github/workflows/ci.yml}"
PLATFORM="${DOCKPIPE_CI_EMULATE_PLATFORM:-ghcr.io/catthehacker/ubuntu:act-22.04}"
WINDOWS_GUEST_PATH="${DOCKPIPE_CI_EMULATE_WINDOWS_GUEST_PATH:-C:\\dockpipe}"
WINDOWS_SYNC_HOST_PATH="${DOCKPIPE_CI_EMULATE_WINDOWS_SYNC_HOST_PATH:-$ROOT}"
WINDOWS_GUEST_COMMAND="${DOCKPIPE_CI_EMULATE_WINDOWS_GUEST_COMMAND:-Set-Location '$WINDOWS_GUEST_PATH'; go test ./...; bash src/scripts/check-templates-core-paths.sh; bash tests/unit-tests/test_clone_worktree_include.sh}"

have() { command -v "$1" >/dev/null 2>&1; }

is_windows_host() {
	[[ "$OSTYPE" == msys* || "$OSTYPE" == cygwin* || "${OS:-}" == "Windows_NT" ]]
}

run_without_dockpipe_workflow_env() {
	(
		unset \
			DOCKPIPE_ARTIFACT_ROOT \
			DOCKPIPE_ASSETS_DIR \
			DOCKPIPE_BIN \
			DOCKPIPE_EVENT_INDEX \
			DOCKPIPE_EVENT_LOG \
			DOCKPIPE_OUTPUT_ROOT \
			DOCKPIPE_PACKAGE_ID \
			DOCKPIPE_PACKAGE_ROOT \
			DOCKPIPE_PACKAGE_STATE_DIR \
			DOCKPIPE_RUN_ID \
			DOCKPIPE_SCRIPT_DIR \
			DOCKPIPE_SOURCE_ROOT \
			DOCKPIPE_STATE_DIR \
			DOCKPIPE_STEP_CWD \
			DOCKPIPE_STEP_OUTPUTS_FILE \
			DOCKPIPE_WORKDIR \
			DOCKPIPE_WORKFLOW_NAME
		"$@"
	)
}

run_windows_subset_on_host() {
	echo "ci-emulate: running the Windows CI subset on the native host"
	run_without_dockpipe_workflow_env go test ./...
	run_without_dockpipe_workflow_env bash src/scripts/check-templates-core-paths.sh
	run_without_dockpipe_workflow_env bash tests/unit-tests/test_clone_worktree_include.sh
}

run_linux_integration_on_host() {
	echo "ci-emulate: running Linux integration tests directly on the Windows host; nested act->docker integration is unreliable on the Windows daemon"
	run_without_dockpipe_workflow_env bash tests/integration-tests/run.sh
}

run_windows_subset_in_vm() {
	echo "ci-emulate: running the Windows CI subset ($WINDOWS_JOB_LABEL) through dockpipe workflow $WINDOWS_WORKFLOW"
	run_without_dockpipe_workflow_env \
		./src/bin/dockpipe --workflow "$WINDOWS_WORKFLOW" --workdir "$ROOT" \
			--var DOCKPIPE_VM_SYNC_HOST_PATH="$WINDOWS_SYNC_HOST_PATH" \
			--var DOCKPIPE_VM_SYNC_GUEST_PATH="$WINDOWS_GUEST_PATH" \
			--var DOCKPIPE_VM_GUEST_COMMAND="$WINDOWS_GUEST_COMMAND" \
			--
}

resolve_act_bin() {
	if [[ -n "${ACT_BIN:-}" && -x "${ACT_BIN}" ]]; then
		printf '%s\n' "$ACT_BIN"
		return 0
	fi
	if have act.exe; then
		command -v act.exe
		return 0
	fi
	if have act; then
		local candidate
		candidate="$(command -v act)"
		if [[ -n "$candidate" && -x "$candidate" ]]; then
			printf '%s\n' "$candidate"
			return 0
		fi
	fi
	if have powershell.exe; then
		powershell.exe -NoProfile -Command "\$cmd = Get-Command -Name 'act' -ErrorAction SilentlyContinue; if (\$cmd) { \$src = \$cmd.Source; if (\$src -and -not \$src.EndsWith('.exe')) { \$exe = \$src + '.exe'; if (Test-Path \$exe) { \$src = \$exe } }; Write-Output \$src }" | tr -d '\r'
		return 0
	fi
	return 1
}

to_windows_path() {
	local path="${1:?path}"
	if have cygpath; then
		cygpath -w "$path"
		return 0
	fi
	printf '%s\n' "$path"
}

if ! have docker; then
	echo "ci-emulate: docker is required" >&2
	exit 1
fi
ACT_BIN="$(resolve_act_bin || true)"
if [[ -z "${ACT_BIN:-}" ]]; then
	cat >&2 <<'MSG'
ci-emulate: act is not installed.

Install nektos/act, then rerun:
  ./src/bin/dockpipe --workflow ci-emulate --workdir . --

Examples:
  Windows: winget install nektos.act
  macOS:   brew install act
  Linux:   see https://github.com/nektos/act#installation
MSG
	exit 1
fi

echo "ci-emulate: event=$EVENT linux-job=$LINUX_JOB windows-job=$WINDOWS_JOB_LABEL workflow=$WORKFLOW platform=$PLATFORM"
if is_windows_host; then
	run_windows_subset_on_host
	run_linux_integration_on_host
	if [[ "${DOCKER_HOST:-}" == npipe:* ]]; then
		echo "ci-emulate: pre-running dockpipe workflow test on the host; using bridge networking because Docker Desktop Linux containers on Windows do not handle workflow host-network mode reliably"
		run_without_dockpipe_workflow_env \
			./src/bin/dockpipe --workflow test --runtime docker --workdir "$ROOT" \
				--var DOCKPIPE_DOCKER_NETWORK=bridge \
				--mount "$(go env GOPATH)/pkg:/go/pkg:rw" \
				--
	fi
else
	run_windows_subset_in_vm
fi
echo "ci-emulate: running the Linux GitHub Actions job locally through act; this may pull runner/action images"
if is_windows_host && have powershell.exe; then
	ACT_BIN_WIN="$(to_windows_path "$ACT_BIN")"
	WORKFLOW_WIN="$(to_windows_path "$WORKFLOW")"
	ARTIFACT_WIN="$(to_windows_path "$ROOT/bin/.dockpipe/act-artifacts")"
	exec env \
		-u DOCKPIPE_ARTIFACT_ROOT \
		-u DOCKPIPE_ASSETS_DIR \
		-u DOCKPIPE_BIN \
		-u DOCKPIPE_EVENT_INDEX \
		-u DOCKPIPE_EVENT_LOG \
		-u DOCKPIPE_OUTPUT_ROOT \
		-u DOCKPIPE_PACKAGE_ID \
		-u DOCKPIPE_PACKAGE_ROOT \
		-u DOCKPIPE_PACKAGE_STATE_DIR \
		-u DOCKPIPE_RUN_ID \
		-u DOCKPIPE_SCRIPT_DIR \
		-u DOCKPIPE_SOURCE_ROOT \
		-u DOCKPIPE_STATE_DIR \
		-u DOCKPIPE_STEP_CWD \
		-u DOCKPIPE_STEP_OUTPUTS_FILE \
		-u DOCKPIPE_WORKDIR \
		-u DOCKPIPE_WORKFLOW_NAME \
		DOCKPIPE_CI_SKIP_ACT_INTEGRATION=1 \
		powershell.exe -NoProfile -Command "& '$ACT_BIN_WIN' '$EVENT' -W '$WORKFLOW_WIN' -j '$LINUX_JOB' -P 'ubuntu-latest=$PLATFORM' --artifact-server-path '$ARTIFACT_WIN'"
fi
exec env \
	-u DOCKPIPE_ARTIFACT_ROOT \
	-u DOCKPIPE_ASSETS_DIR \
	-u DOCKPIPE_BIN \
	-u DOCKPIPE_EVENT_INDEX \
	-u DOCKPIPE_EVENT_LOG \
	-u DOCKPIPE_OUTPUT_ROOT \
	-u DOCKPIPE_PACKAGE_ID \
	-u DOCKPIPE_PACKAGE_ROOT \
	-u DOCKPIPE_PACKAGE_STATE_DIR \
	-u DOCKPIPE_RUN_ID \
	-u DOCKPIPE_SCRIPT_DIR \
	-u DOCKPIPE_SOURCE_ROOT \
	-u DOCKPIPE_STATE_DIR \
	-u DOCKPIPE_STEP_CWD \
	-u DOCKPIPE_STEP_OUTPUTS_FILE \
	-u DOCKPIPE_WORKDIR \
	-u DOCKPIPE_WORKFLOW_NAME \
	"$ACT_BIN" "$EVENT" \
		-W "$WORKFLOW" \
		-j "$LINUX_JOB" \
		-P "ubuntu-latest=$PLATFORM" \
		--artifact-server-path "$ROOT/bin/.dockpipe/act-artifacts"
