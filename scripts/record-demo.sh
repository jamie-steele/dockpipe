#!/usr/bin/env bash
# Record a short terminal demo: dockpipe --workflow test --runtime docker → demo/dockpipe-demo.gif
# Requires: asciinema, agg, Docker, and a built CLI (make build).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

DEMO_DIR="${ROOT}/demo"
CAST="${DEMO_DIR}/dockpipe-demo.cast"
GIF="${DEMO_DIR}/dockpipe-demo.gif"

die() { echo "record-demo: $*" >&2; exit 1; }

print_record_deps_help() {
	cat >&2 <<'EOT'

Missing tools for demo recording. Install:

  asciinema   Debian/Ubuntu/Pop!_OS:  sudo apt install asciinema
              Fedora:                 sudo dnf install asciinema
              macOS:                  brew install asciinema

  agg         GIF renderer (not always packaged):
              https://github.com/asciinema/agg/releases  (download binary → chmod +x → put on PATH)
              or:  cargo install --locked --git https://github.com/asciinema/agg

Then re-run:  make demo-record
EOT
}

command -v asciinema >/dev/null 2>&1 || { echo "record-demo: asciinema not found." >&2; print_record_deps_help; exit 1; }
command -v agg >/dev/null 2>&1 || { echo "record-demo: agg not found." >&2; print_record_deps_help; exit 1; }
command -v docker >/dev/null 2>&1 || die "install Docker and ensure it is running"
docker info >/dev/null 2>&1 || die "Docker daemon not reachable — start Docker"

[[ -x "${ROOT}/bin/dockpipe.bin" ]] || [[ -f "${ROOT}/bin/dockpipe" ]] || die "run \`make build\` first (or: make demo-record)"

mkdir -p "$DEMO_DIR"

INNER="$(mktemp)"
trap 'rm -f "$INNER"' EXIT
cat > "$INNER" <<EOF
#!/usr/bin/env bash
set -euo pipefail
cd "$ROOT"
export DOCKPIPE_REPO_ROOT="$ROOT"
printf '%s\n' '$ dockpipe --workflow test --runtime docker'
exec ./bin/dockpipe --workflow test --runtime docker
EOF
chmod +x "$INNER"

echo "Recording to ${CAST} …"
asciinema rec --overwrite "$CAST" -c "$INNER"

echo "Rendering GIF with agg …"
# Readable when scaled down: slightly larger font, modest terminal size, snappier playback
agg \
  --theme asciinema \
  --font-size 18 \
  --cols 82 \
  --rows 22 \
  --fps 12 \
  --speed 1.15 \
  "$CAST" \
  "$GIF"

echo ""
echo "Saved: $GIF"
echo "Intermediate cast (optional): $CAST"
