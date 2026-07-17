#!/usr/bin/env bash

dorkpipe_test_tool_is_runnable() {
	local tool="${1:?tool}"
	shift || true
	command -v "$tool" >/dev/null 2>&1 || return 1
	"$tool" "$@" >/dev/null 2>&1
}

dorkpipe_test_require_go() {
	if ! dorkpipe_test_tool_is_runnable go version; then
		echo "${1:-dorkpipe package test}: skip (go not runnable)" >&2
		exit 0
	fi
}

dorkpipe_test_init_go_cache() {
	local root="${1:?repo root}"
	export GOCACHE="${GOCACHE:-${root}/bin/.dockpipe/build/go-cache}"
	export GOTMPDIR="${GOTMPDIR:-${root}/bin/.dockpipe/build/go-tmp}"
	mkdir -p "$GOCACHE" "$GOTMPDIR"
}

dorkpipe_test_tmp_root() {
	local root="${1:?repo root}"
	local tmp_root="${DORKPIPE_PACKAGE_TEST_TMPDIR:-${root}/bin/.dockpipe/tmp/package-tests}"
	mkdir -p "$tmp_root"
	printf '%s\n' "$tmp_root"
}

dorkpipe_test_mktemp_dir() {
	local root="${1:?repo root}"
	local tmp_root
	tmp_root="$(dorkpipe_test_tmp_root "$root")"
	mktemp -d "${tmp_root}/tmp.XXXXXXXXXX"
}

dorkpipe_test_assert() {
	local root="${1:?repo root}"
	shift
	dorkpipe_test_require_go "dorkpipe_test_assert"
	dorkpipe_test_init_go_cache "$root"
	(
		cd "$root/packages/dorkpipe/lib"
		go run ./cmd/test-assert "$@"
	)
}
