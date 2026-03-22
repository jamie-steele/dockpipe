# Demo recording (developer / social)

Install **asciinema** + **agg** (best effort, no sudo):

```bash
make install-record-deps
```

(`make dev-deps` runs this after CI tools — see repo README → Development.)

Recordings use **`--workflow test-demo`**: **tests → `go vet` → security brief** (`make demo-record`). **This repo’s CI** uses **`--workflow test`** (**`go vet`** in Docker only; **govulncheck**/**gosec** on the host). The **long** variant also runs **`dockpipe --version`** first.

**What you see (`test-demo`):** **isolation** banner → **`go test`** → **`go vet`** → **security brief**. The brief is **labeled** bundled copy — **not** a live LLM. **`make demo-record`** mounts **`$(go env GOPATH)/pkg` → `/go/pkg`** for module reuse.

## One command (builds both GIFs)

From the repo root (or from `scripts/` — same Makefile forwards to the root):

```bash
make demo-record
```

Output:

- `demo/dockpipe-demo-short.gif` — compact layout (quick social / thumbnail)
- `demo/dockpipe-demo-long.gif` — wider terminal + version + workflow (longer story)
- Matching `.cast` files — asciinema sources (can delete after rendering)

Record only one variant:

```bash
make demo-record-short
make demo-record-long
```

## Prerequisites

- **Docker** running
- **asciinema** — terminal recorder
- **agg** — `.cast` → GIF ([releases](https://github.com/asciinema/agg/releases))
- **make build** — `make demo-record` builds the CLI first

### Pop!_OS / Ubuntu / Debian

```bash
sudo apt update
sudo apt install -y asciinema
```

**agg** is often not in apt. Either download a release binary from [asciinema/agg releases](https://github.com/asciinema/agg/releases), put it on your `PATH` as `agg`, or:

```bash
cargo install --locked --git https://github.com/asciinema/agg
```

### macOS

```bash
brew install asciinema
```

Install **agg** from [GitHub releases](https://github.com/asciinema/agg/releases) or `cargo install --locked --git https://github.com/asciinema/agg` (same as Linux).

## Manual run

```bash
make build
bash scripts/record-demo.sh all    # or: short | long
```
