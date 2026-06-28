#!/usr/bin/env bash
# Pipeon — local-first, repo-aware chat helper (Ollama). Gated by DOCKPIPE_PIPEON and version; see README.md
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
ROOT="$(dockpipe get workdir)"
if [[ -n "${DOCKPIPE_SDK_SH:-}" && -f "$DOCKPIPE_SDK_SH" ]]; then
	# shellcheck source=/dev/null
	source "$DOCKPIPE_SDK_SH"
	dockpipe_sdk_refresh "$ROOT"
else
	eval "$(dockpipe sdk --workdir "$ROOT")"
fi
# shellcheck source=lib/enable.sh
source "$SCRIPT_DIR/lib/enable.sh"

pipeon_bash_bin() {
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

PIPEON_BASH_BIN="$(pipeon_bash_bin)" || {
	echo "pipeon: bash executable not found for nested helper scripts" >&2
	exit 1
}

cmd="${1:-help}"
shift || true

case "$cmd" in
help | -h | --help)
	cat <<EOF
Pipeon (local IDE assistant helper)

  Feature flag: DOCKPIPE_PIPEON=1
  Version gate: default min ${PIPEON_MIN_VERSION} — set DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1 to try earlier.

Commands:
  bundle          Write bin/.dockpipe/packages/pipeon/pipeon-context.md from repo artifacts
  chat [text]     Ask Ollama (compatibility snapshot optional; requires ollama serve). Or: pipeon chat < file.txt
  status          Show enablement, version gate, and artifact presence

Environment:
  OLLAMA_HOST          Default http://127.0.0.1:11434
  PIPEON_OLLAMA_MODEL  Default llama3.2 (or DOCKPIPE_OLLAMA_MODEL)

See: assets/docs/pipeon-ide-experience.md and assets/scripts/README.md (next to this script under the pipeon resolver).
EOF
	;;
bundle)
	pipeon_check_enabled "$ROOT" || exit $?
	"$PIPEON_BASH_BIN" "$SCRIPT_DIR/bundle-context.sh"
	;;
chat)
	pipeon_check_enabled "$ROOT" || exit $?
	"$PIPEON_BASH_BIN" "$SCRIPT_DIR/chat.sh" "$@"
	;;
status)
	echo "DOCKPIPE_PIPEON=${DOCKPIPE_PIPEON:-<unset>}"
	echo "DOCKPIPE_PIPEON_MIN_VERSION=${DOCKPIPE_PIPEON_MIN_VERSION:-$PIPEON_MIN_VERSION}"
	echo "DOCKPIPE_PIPEON_ALLOW_PRERELEASE=${DOCKPIPE_PIPEON_ALLOW_PRERELEASE:-<unset>}"
	v="$(pipeon_version_from_repo "$ROOT")"
	echo "repo VERSION: $v"
	if [[ "${DOCKPIPE_PIPEON:-}" == "1" || "${DOCKPIPE_PIPEON:-}" == "true" ]]; then
		echo "gate: DOCKPIPE_PIPEON ok"
	else
		echo "gate: blocked — set DOCKPIPE_PIPEON=1"
	fi
	if [[ "${DOCKPIPE_PIPEON_ALLOW_PRERELEASE:-}" == "1" ]]; then
		echo "gate: prerelease override ON"
	elif semver_lt "$v" "${DOCKPIPE_PIPEON_MIN_VERSION:-$PIPEON_MIN_VERSION}"; then
		echo "gate: version < ${DOCKPIPE_PIPEON_MIN_VERSION:-$PIPEON_MIN_VERSION} — set DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1 until release"
	else
		echo "gate: version ok for Pipeon"
	fi
	PIPEON_STATE_DIR="$(dockpipe_sdk path package pipeon)"
	CI_FINDINGS="${DOCKPIPE_CI_ANALYSIS_DIR:?DOCKPIPE_CI_ANALYSIS_DIR is required}/findings.json"
	INSIGHTS="$(dockpipe_sdk path state analysis insights.json)"
	DORKPIPE_RUN="$(dockpipe_sdk path package dorkpipe run.json)"
	for p in "$PIPEON_STATE_DIR/pipeon-context.md" "$CI_FINDINGS" "$INSIGHTS" "$DORKPIPE_RUN"; do
		if [[ -f "$p" ]]; then
			echo "present: $p"
		else
			echo "absent:  $p"
		fi
	done
	;;
*)
	echo "pipeon: unknown command: $cmd (try: pipeon help)" >&2
	exit 1
	;;
esac
