#!/usr/bin/env bash
# Prepare generated DorkPipe assets that must be present before go:embed builds.
set -euo pipefail

cmd="${1:-prepare}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RESOLVER_DIR="${REPO_ROOT}/packages/dorkpipe/resolvers/dorkpipe"
HOOK="${RESOLVER_DIR}/assets/scripts/package-consumer-artifacts.sh"
TOOLING_BIN="${RESOLVER_DIR}/assets/tooling/bin"
MARKER="${TOOLING_BIN}/.dockpipe-generated"
COMPILE_WORKDIR="${REPO_ROOT}/bin/.dockpipe/build/embedded-dorkpipe-assets"

clean_generated() {
	if [[ -f "${MARKER}" || "${DORKPIPE_CLEAN_EMBEDDED_DORKPIPE_ASSETS_FORCE:-0}" == "1" ]]; then
		rm -rf "${TOOLING_BIN}"
	fi
}

case "${cmd}" in
prepare)
	if [[ ! -f "${HOOK}" ]]; then
		echo "prepare embedded dorkpipe assets: missing ${HOOK}" >&2
		exit 1
	fi
	rm -rf "${TOOLING_BIN}"
	mkdir -p "${COMPILE_WORKDIR}"
	DOCKPIPE_COMPILE_KIND=resolver \
	DOCKPIPE_COMPILE_WORKDIR="${COMPILE_WORKDIR}" \
	DOCKPIPE_COMPILE_SOURCE_DIR="${RESOLVER_DIR}" \
	DOCKPIPE_COMPILE_STAGING_DIR="${RESOLVER_DIR}" \
		bash "${HOOK}"
	touch "${MARKER}"
	for path in \
		"${TOOLING_BIN}/linux/dockpipe" \
		"${TOOLING_BIN}/linux/dorkpipe" \
		"${TOOLING_BIN}/linux/mcpd"
	do
		if [[ ! -s "${path}" ]]; then
			echo "prepare embedded dorkpipe assets: missing generated binary ${path}" >&2
			exit 1
		fi
		chmod +x "${path}" 2>/dev/null || true
	done
	;;
clean)
	clean_generated
	;;
*)
	echo "usage: $0 prepare|clean" >&2
	exit 1
	;;
esac
