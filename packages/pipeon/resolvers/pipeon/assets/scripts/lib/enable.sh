#!/usr/bin/env bash
# Pipeon feature gate: explicit enable + minimum version (default 0.6.5) unless prerelease allowed.
# Source from other scripts:  source "$(dirname ...)/lib/enable.sh"
set -euo pipefail
ROOT="$(dockpipe get workdir)"

# DOCKPIPE_PIPEON=1 (or "true") — required to run Pipeon commands.
# DOCKPIPE_PIPEON_MIN_VERSION — semver string, default 0.6.5 (release when Pipeon is officially on).
# DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1 — allow running when repo VERSION < MIN (dev / CI / early adopters).

PIPEON_MIN_VERSION="${DOCKPIPE_PIPEON_MIN_VERSION:-0.6.5}"

pipeon_version_from_repo() {
	local root="${1:-}"
	if [[ -z "$root" ]]; then
		root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
	fi
	if [[ -f "$root/VERSION" ]]; then
		tr -d ' \t\r\n' <"$root/VERSION"
		return
	fi
	local dockpipe_bin
	dockpipe_bin="${DOCKPIPE_BIN:-}"
	if [[ -z "$dockpipe_bin" ]]; then
		dockpipe_bin="$(dockpipe get dockpipe_bin 2>/dev/null || true)"
	fi
	if [[ -n "$dockpipe_bin" ]]; then
		"$dockpipe_bin" --version 2>/dev/null | head -1 | tr -d ' \t\r\n'
		return
	fi
	echo "0.0.0"
}

# Return 0 if $1 < $2 (semver), else 1. Uses sort -V.
semver_lt() {
	local a="$1" b="$2"
	[[ "$(printf '%s\n' "$a" "$b" | sort -V | head -1)" == "$a" ]] && [[ "$a" != "$b" ]]
}

pipeon_check_enabled() {
	local root="${1:-}"
	local v
	v="$(pipeon_version_from_repo "$root")"

	if [[ "${DOCKPIPE_PIPEON:-}" != "1" && "${DOCKPIPE_PIPEON:-}" != "true" ]]; then
		cat >&2 <<'EOF'
pipeon: disabled (feature-flag).

  Export DOCKPIPE_PIPEON=1 to enable. Pipeon stays local-first (Ollama by default); see assets/docs/pipeon-ide-experience.md under the pipeon resolver.

EOF
		return 2
	fi

	if [[ "${DOCKPIPE_PIPEON_ALLOW_PRERELEASE:-}" == "1" ]]; then
		return 0
	fi

	if semver_lt "$v" "$PIPEON_MIN_VERSION"; then
		cat >&2 <<EOF
pipeon: repo version is ${v}; Pipeon is gated until ${PIPEON_MIN_VERSION}.

  Set DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1 to try before release (developers only), or bump VERSION when shipping.

EOF
		return 3
	fi
	return 0
}
