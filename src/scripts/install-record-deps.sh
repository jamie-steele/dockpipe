#!/usr/bin/env bash
# Optional: asciinema + agg for `make demo-record` (see release/demo/README.md).
# Best-effort user-level installs; does not use sudo. Exits 0 even if tools remain missing.
set -uo pipefail

case "$(uname -s)" in
Linux) ;;
Darwin) ;;
*)
	echo "install-record-deps: unsupported OS (Linux and macOS only)." >&2
	exit 0
	;;
esac

# So we detect tools just installed to ~/.local/bin or ~/.cargo/bin in this shell
export PATH="${HOME}/.local/bin:${HOME}/.cargo/bin:${PATH}"

have() { command -v "$1" >/dev/null 2>&1; }

# Download official agg binary from GitHub releases (no Rust toolchain required).
install_agg_release_binary() {
	local bin_dir="${HOME}/.local/bin"
	local machine os asset url
	machine="$(uname -m)"
	os="$(uname -s)"
	asset=""
	case "$os" in
	Linux)
		case "$machine" in
		x86_64) asset="agg-x86_64-unknown-linux-gnu" ;;
		aarch64 | arm64) asset="agg-aarch64-unknown-linux-gnu" ;;
		armv7l | armv6l) asset="agg-arm-unknown-linux-gnueabihf" ;;
		*) return 1 ;;
		esac
		;;
	Darwin)
		case "$machine" in
		x86_64) asset="agg-x86_64-apple-darwin" ;;
		aarch64 | arm64) asset="agg-aarch64-apple-darwin" ;;
		*) return 1 ;;
		esac
		;;
	*) return 1 ;;
	esac
	mkdir -p "$bin_dir"
	url="https://github.com/asciinema/agg/releases/latest/download/${asset}"
	echo "agg: downloading ${asset} from GitHub releases..."
	if have curl; then
		curl -fsSL "$url" -o "${bin_dir}/agg.new" || return 1
	elif have wget; then
		wget -q -O "${bin_dir}/agg.new" "$url" || return 1
	else
		echo "agg: need curl or wget to download release binary" >&2
		return 1
	fi
	mv -f "${bin_dir}/agg.new" "${bin_dir}/agg"
	chmod +x "${bin_dir}/agg"
}

echo "checking asciinema..."
if have asciinema; then
	echo "asciinema: already installed ($(command -v asciinema))"
else
	echo "asciinema: installing (user-level)..."
	installed=false
	if have pipx; then
		pipx install asciinema && installed=true || true
	fi
	if [[ "$installed" != true ]] && have python3 && python3 -m pip --version >/dev/null 2>&1; then
		if python3 -m pip install --user --quiet asciinema; then
			installed=true
		elif [[ "$(uname -s)" == Linux ]] && python3 -m pip install --user --quiet --break-system-packages asciinema; then
			# Debian/Ubuntu/Pop!_OS: PEP 668 blocks plain --user without this flag
			installed=true
		fi
	fi
	if [[ "$installed" != true ]] && [[ "$(uname -s)" == Darwin ]] && have brew; then
		brew install asciinema && installed=true || true
	fi
	if have asciinema; then
		echo "asciinema: ok ($(command -v asciinema))"
	else
		echo "asciinema: still missing — try one of:" >&2
		echo "  sudo apt install asciinema          # Debian/Ubuntu/Pop!_OS" >&2
		echo "  sudo apt install pipx && pipx install asciinema   # user install, no system pip" >&2
		echo "  brew install asciinema              # macOS" >&2
	fi
fi

echo "checking agg..."
if have agg; then
	echo "agg: already installed ($(command -v agg))"
else
	if install_agg_release_binary && have agg; then
		echo "agg: ok ($(command -v agg))"
	elif have cargo; then
		echo "agg: installing via cargo (user-level)..."
		if cargo install --locked --git https://github.com/asciinema/agg && have agg; then
			echo "agg: ok ($(command -v agg))"
		else
			echo "agg: cargo install failed — see https://github.com/asciinema/agg/releases" >&2
		fi
	fi
	if ! have agg; then
		echo "agg: still missing — install curl or wget and re-run, or download from https://github.com/asciinema/agg/releases" >&2
	fi
fi

for d in "$HOME/.local/bin" "$HOME/.cargo/bin"; do
	if [[ -d "$d" ]] && [[ ":$PATH:" != *":$d:"* ]]; then
		echo "Note: add to PATH if needed: export PATH=\"$d:\$PATH\""
	fi
done

echo ""
echo "Demo recording dependencies step finished (optional tools for make demo-record)."
