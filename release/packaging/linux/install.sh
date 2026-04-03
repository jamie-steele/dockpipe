#!/usr/bin/env sh
# Install dockpipe on Linux from GitHub Releases (deb, apk, rpm, or Arch package).
#
#   curl -fsSL https://raw.githubusercontent.com/jamie-steele/dockpipe/master/release/packaging/linux/install.sh | sh
#
# Optional env:
#   DOCKPIPE_VERSION=1.2.3   Pin tag (default: latest release)
#   DOCKPIPE_REPO=owner/repo  Fork releases
set -eu

REPO="${DOCKPIPE_REPO:-jamie-steele/dockpipe}"
VERSION="${DOCKPIPE_VERSION:-}"
TMP="${TMPDIR:-/tmp}/dockpipe-install-$$"
mkdir -p "$TMP"
trap 'rm -rf "$TMP"' EXIT INT TERM

if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -qO- "$1"; }
else
  echo "Need curl or wget" >&2
  exit 1
fi

if [ -z "$VERSION" ]; then
  VERSION="$(fetch "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -1)"
fi
VERSION="${VERSION#v}"
if [ -z "$VERSION" ]; then
  echo "Could not determine release version" >&2
  exit 1
fi

arch="$(uname -m)"
case "$arch" in
  x86_64) goarch=amd64 ;;
  aarch64 | arm64) goarch=arm64 ;;
  *) echo "Unsupported architecture: $arch (need x86_64 or aarch64)" >&2; exit 1 ;;
esac

base="https://github.com/${REPO}/releases/download/v${VERSION}"
id=
if [ -f /etc/os-release ]; then
  # shellcheck source=/dev/null
  . /etc/os-release
  id="${ID:-}"
fi

run_root() {
  if [ "$(id -u)" = 0 ]; then
    "$@"
  else
    sudo "$@"
  fi
}

case "$id" in
  alpine)
    pkg="dockpipe_${VERSION}_linux_${goarch}.apk"
    fetch "${base}/${pkg}" >"$TMP/$pkg"
    run_root apk add --allow-untrusted "$TMP/$pkg"
    ;;
  fedora | rhel | centos | rocky | almalinux)
    pkg="dockpipe_${VERSION}_linux_${goarch}.rpm"
    fetch "${base}/${pkg}" >"$TMP/$pkg"
    if command -v dnf >/dev/null 2>&1; then
      run_root dnf install -y "$TMP/$pkg"
    elif command -v yum >/dev/null 2>&1; then
      run_root yum install -y "$TMP/$pkg"
    else
      run_root rpm -Uvh "$TMP/$pkg"
    fi
    ;;
  arch | archlinux | endeavouros | manjaro)
    pkg="dockpipe_${VERSION}_linux_${goarch}.pkg.tar.zst"
    fetch "${base}/${pkg}" >"$TMP/$pkg"
    run_root pacman -U --noconfirm "$TMP/$pkg"
    ;;
  debian | ubuntu | pop | linuxmint | zorin)
    pkg="dockpipe_${VERSION}_${goarch}.deb"
    fetch "${base}/${pkg}" >"$TMP/$pkg"
    run_root dpkg -i "$TMP/$pkg" || run_root apt-get install -f -y
    ;;
  *)
    echo "Unrecognized distro ID '${id:-unknown}' — installing portable tarball to ~/.local/bin"
    tgz="dockpipe_${VERSION}_linux_${goarch}.tar.gz"
    fetch "${base}/${tgz}" >"$TMP/$tgz"
    mkdir -p "${HOME}/.local/bin"
    tar -xzf "$TMP/$tgz" -C "${HOME}/.local/bin" dockpipe
    chmod +x "${HOME}/.local/bin/dockpipe"
    shell="${SHELL:-/bin/sh}"
    case "$shell" in
      */bash | */zsh) rc="${HOME}/.bashrc" ;;
      *) rc="${HOME}/.profile" ;;
    esac
    if [ -f "$rc" ] && ! grep -Fq '.local/bin' "$rc" 2>/dev/null; then
      printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >>"$rc"
    fi
    echo "Add ~/.local/bin to PATH for this session: export PATH=\"\$HOME/.local/bin:\$PATH\""
    ;;
esac

echo "Installed dockpipe ${VERSION}. Ensure Docker and bash are available: dockpipe doctor"
