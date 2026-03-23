#!/usr/bin/env bash
# Pipeon — local-first, repo-aware chat helper (Ollama). Gated by DOCKPIPE_PIPEON and version; see README.md
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
# shellcheck source=lib/enable.sh
source "$SCRIPT_DIR/lib/enable.sh"

cmd="${1:-help}"
shift || true

case "$cmd" in
help | -h | --help)
	cat <<EOF
Pipeon (local IDE assistant helper)

  Feature flag: DOCKPIPE_PIPEON=1
  Version gate: default min ${PIPEON_MIN_VERSION} — set DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1 to try earlier.

Commands:
  bundle          Write .dockpipe/pipeon-context.md from repo artifacts
  chat [text]     Ask Ollama (requires bundle + ollama serve). Or: pipeon chat < file.txt
  status          Show enablement, version gate, and artifact presence

Environment:
  OLLAMA_HOST          Default http://127.0.0.1:11434
  PIPEON_OLLAMA_MODEL  Default llama3.2 (or DOCKPIPE_OLLAMA_MODEL)

See: src/pipeon/docs/pipeon-ide-experience.md  src/pipeon/scripts/README.md
EOF
	;;
bundle)
	pipeon_check_enabled "$ROOT" || exit $?
	bash "$SCRIPT_DIR/bundle-context.sh"
	;;
chat)
	pipeon_check_enabled "$ROOT" || exit $?
	if [[ ! -f "$ROOT/.dockpipe/pipeon-context.md" ]]; then
		bash "$SCRIPT_DIR/bundle-context.sh"
	fi
	bash "$SCRIPT_DIR/chat.sh" "$@"
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
	for p in "$ROOT/.dockpipe/pipeon-context.md" "$ROOT/.dockpipe/ci-analysis/findings.json" "$ROOT/.dockpipe/analysis/insights.json" "$ROOT/.dorkpipe/run.json"; do
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
