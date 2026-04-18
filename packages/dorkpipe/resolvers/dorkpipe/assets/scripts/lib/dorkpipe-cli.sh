#!/usr/bin/env bash

dorkpipe_script_repo_root() {
	local script_dir="${1:?script_dir is required}"
	cd "$script_dir" && git rev-parse --show-toplevel 2>/dev/null || true
}

dorkpipe_script_bootstrap() {
	local script_dir="${1:?script_dir is required}"
	export DOCKPIPE_WORKDIR="${DOCKPIPE_WORKDIR:-$(pwd)}"

	local repo_root
	repo_root="$(dorkpipe_script_repo_root "$script_dir")"
	if [[ -n "$repo_root" && -f "$repo_root/src/core/assets/scripts/lib/dockpipe-sdk.sh" ]]; then
		# shellcheck source=/dev/null
		source "$repo_root/src/core/assets/scripts/lib/dockpipe-sdk.sh"
		return 0
	fi

	eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"
}

dorkpipe_script_exec_cli() {
	local script_dir="${1:?script_dir is required}"
	shift

	local repo_root dorkpipe_bin
	repo_root="$(dorkpipe_script_repo_root "$script_dir")"
	if command -v go >/dev/null 2>&1 && dorkpipe_script_should_use_go_run "$repo_root"; then
		cd "$repo_root/packages/dorkpipe/lib"
		exec go run ./cmd/dorkpipe "$@"
	fi

	dorkpipe_bin="$(dorkpipe_script_resolve_bin "$repo_root")" || dockpipe_sdk die "dorkpipe not found; build the maintainer tool or install it on PATH"
	exec "$dorkpipe_bin" "$@"
}

dorkpipe_script_resolve_bin() {
	local repo_root="${1:-}"
	if [[ -n "$repo_root" ]]; then
		local built_bin="$repo_root/packages/dorkpipe/bin/dorkpipe"
		if [[ -x "$built_bin" ]]; then
			printf '%s\n' "$built_bin"
			return 0
		fi
	fi

	if command -v dorkpipe >/dev/null 2>&1; then
		command -v dorkpipe
		return 0
	fi

	if command -v go >/dev/null 2>&1 && [[ -n "$repo_root" && -f "$repo_root/packages/dorkpipe/lib/go.mod" ]]; then
		local shim
		shim="$(mktemp)"
		cat >"$shim" <<EOF
#!/usr/bin/env bash
set -euo pipefail
cd "$repo_root/packages/dorkpipe/lib"
exec go run ./cmd/dorkpipe "\$@"
EOF
		chmod +x "$shim"
		printf '%s\n' "$shim"
		return 0
	fi

	return 1
}

dorkpipe_script_should_use_go_run() {
	local repo_root="${1:-}"
	[[ -n "$repo_root" ]] || return 1
	[[ -f "$repo_root/packages/dorkpipe/lib/go.mod" ]] || return 1

	local built_bin="$repo_root/packages/dorkpipe/bin/dorkpipe"
	if [[ ! -x "$built_bin" ]]; then
		return 0
	fi

	find "$repo_root/packages/dorkpipe/lib" -type f \( -name '*.go' -o -name '*.json' \) -newer "$built_bin" -print -quit | grep -q .
}
