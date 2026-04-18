#!/usr/bin/env bash
# Record terminal demos: dockpipe test-demo (go test → vet → review prep → local Ollama → codex exec) → release/demo/dockpipe-demo-{short,long}.gif
# The test-demo review step is resolver-driven (codex) inside runtime docker; Codex inner sandbox is
# off via workflow YAML (--dangerously-bypass-approvals-and-sandbox) — see templates/core/resolvers/codex/README.md
# Requires OPENAI_API_KEY (and/or CODEX_API_KEY) for the final Codex step (export or .env before recording).
# Requires: asciinema, agg, Docker, and a built CLI (make build).
#
# Usage: bash src/scripts/record-demo.sh [short|long|all]
#   short — compact GIF (social / quick share)
#   long  — wider terminal + version line + workflow (fuller showcase)
#   all   — both (default)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

DEMO_DIR="${ROOT}/demo"

die() { echo "record-demo: $*" >&2; exit 1; }

MODE="${1:-all}"
case "$MODE" in
short | long | all) ;;
*)
	die "usage: bash src/scripts/record-demo.sh [short|long|all]"
	;;
esac

print_record_deps_help() {
	cat >&2 <<'EOT'

Missing tools for demo recording. Install:

  First try (from repo root):  make install-record-deps
  If that leaves tools missing, install manually:

  asciinema   Debian/Ubuntu/Pop!_OS:  sudo apt install asciinema
              or:  sudo apt install pipx && pipx install asciinema   (user-level)
              Fedora:                 sudo dnf install asciinema
              macOS:                  brew install asciinema

  agg         GIF renderer (not always packaged):
              https://github.com/asciinema/agg/releases  (download binary → chmod +x → put on PATH)
              or:  cargo install --locked --git https://github.com/asciinema/agg

Ensure ~/.local/bin and ~/.cargo/bin are on your PATH if you used pip/cargo.

Then re-run:  make demo-record
(  make dev-deps  installs CI tools plus the demo-record helpers. )
EOT
}

command -v asciinema >/dev/null 2>&1 || { echo "record-demo: asciinema not found." >&2; print_record_deps_help; exit 1; }
command -v agg >/dev/null 2>&1 || { echo "record-demo: agg not found." >&2; print_record_deps_help; exit 1; }
command -v docker >/dev/null 2>&1 || die "install Docker and ensure it is running"
docker info >/dev/null 2>&1 || die "Docker daemon not reachable — start Docker"

[[ -x "${ROOT}/src/bin/dockpipe.bin" ]] || [[ -f "${ROOT}/src/bin/dockpipe" ]] || die "run \`make build\` first (or: make demo-record)"
command -v go >/dev/null 2>&1 || die "need Go on PATH (go mod download + module cache mount for the test-demo workflow)"

mkdir -p "$DEMO_DIR"

write_inner_short() {
	local f="$1"
	cat >"$f" <<EOF
#!/usr/bin/env bash
set -euo pipefail
cd "$ROOT"
export DOCKPIPE_REPO_ROOT="$ROOT"
go mod download
GOPKG="\$(go env GOPATH)/pkg"
printf '%s\n' '$ dockpipe --workflow test-demo --resolver codex --runtime docker --workdir . --mount "$(go env GOPATH)/pkg:/go/pkg:rw"'
exec ./src/bin/dockpipe --workflow test-demo --resolver codex --runtime docker --workdir "$ROOT" --mount "\${GOPKG}:/go/pkg:rw" --
EOF
}

write_inner_long() {
	local f="$1"
	cat >"$f" <<EOF
#!/usr/bin/env bash
set -euo pipefail
cd "$ROOT"
export DOCKPIPE_REPO_ROOT="$ROOT"
go mod download
GOPKG="\$(go env GOPATH)/pkg"
printf '%s\n' '$ dockpipe --version'
./src/bin/dockpipe --version
printf '%s\n' '$ dockpipe --workflow test-demo --resolver codex --runtime docker --workdir . --mount "$(go env GOPATH)/pkg:/go/pkg:rw"'
exec ./src/bin/dockpipe --workflow test-demo --resolver codex --runtime docker --workdir "$ROOT" --mount "\${GOPKG}:/go/pkg:rw" --
EOF
}

render_gif() {
	local cast="$1"
	local gif="$2"
	shift 2
	agg \
		--theme asciinema \
		"$@" \
		"$cast" \
		"$gif"
}

record_variant() {
	local variant="$1"
	local inner cast gif
	inner="$(mktemp)"
	trap 'rm -f "$inner"' RETURN
	chmod +x "$inner"

	case "$variant" in
	short)
		write_inner_short "$inner"
		cast="${DEMO_DIR}/dockpipe-demo-short.cast"
		gif="${DEMO_DIR}/dockpipe-demo-short.gif"
		echo "Recording (short) → ${cast} …"
		asciinema rec --overwrite "$cast" -c "$inner"
		echo "Rendering GIF (short, compact) …"
		# Tight layout for thumbnails / quick shares
		render_gif "$cast" "$gif" \
			--font-size 16 \
			--cols 72 \
			--rows 18 \
			--fps-cap 14 \
			--speed 1.2
		echo "Saved: $gif"
		;;
	long)
		write_inner_long "$inner"
		cast="${DEMO_DIR}/dockpipe-demo-long.cast"
		gif="${DEMO_DIR}/dockpipe-demo-long.gif"
		echo "Recording (long) → ${cast} …"
		asciinema rec --overwrite "$cast" -c "$inner"
		echo "Rendering GIF (long, readable) …"
		# Wider + taller: version line + full workflow output
		render_gif "$cast" "$gif" \
			--font-size 18 \
			--cols 92 \
			--rows 30 \
			--fps-cap 10 \
			--speed 1.0
		echo "Saved: $gif"
		;;
	*)
		die "internal: bad variant $variant"
		;;
	esac
	echo "Intermediate cast (optional): $cast"
	echo ""
}

case "$MODE" in
all)
	record_variant short
	record_variant long
	echo "Done: release/demo/dockpipe-demo-short.gif and release/demo/dockpipe-demo-long.gif"
	;;
short)
	record_variant short
	;;
long)
	record_variant long
	;;
esac
