#!/usr/bin/env bash
# Contributors only: installs govulncheck + gosec (same as .github/workflows/ci.yml).
# golangci-lint is not used in this repo — omitted on purpose.
# Does not install Docker. See README → Development.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

case "$(uname -s)" in
Linux) ;;
Darwin) ;;
*)
	echo "install-deps: unsupported OS (Linux and macOS only)." >&2
	exit 1
	;;
esac

echo "checking Go..."
if ! command -v go >/dev/null 2>&1; then
	echo "install-deps: Go is not installed." >&2
	echo "Install from https://go.dev/dl/ — then re-run this script. Toolchain in go.mod is applied when you run go in this repo." >&2
	exit 1
fi
go version
go list -e . >/dev/null

echo "checking make..."
if command -v make >/dev/null 2>&1; then
	echo "make: already installed ($(command -v make))"
else
	echo "install-deps: make not found." >&2
	echo "Install with your OS (e.g. build-essential on Debian/Ubuntu, Xcode Command Line Tools on macOS)." >&2
	exit 1
fi

GOBIN="$(go env GOPATH)/bin"
export PATH="$GOBIN:$PATH"

install_go_tool() {
	local bin="$1"
	local pkg="$2"
	echo "checking ${bin}..."
	if command -v "$bin" >/dev/null 2>&1; then
		echo "${bin}: already installed ($GOBIN/${bin})"
		return
	fi
	echo "${bin}: installing..."
	go install "$pkg"
}

install_go_tool govulncheck golang.org/x/vuln/cmd/govulncheck@latest
install_go_tool gosec github.com/securego/gosec/v2/cmd/gosec@latest

if [[ ":$PATH:" != *":$GOBIN:"* ]]; then
	echo ""
	echo "Note: add Go bin to PATH: export PATH=\"$GOBIN:\$PATH\""
fi

echo ""
echo "Development dependencies installed."
