# Demo recording (developer / social)

Install **asciinema** + **agg** (best effort, no sudo):

```bash
make install-record-deps
```

(`make dev-deps` runs this after CI tools — see repo README → Development.)

The checked-in recordings are historical demo artifacts. Regenerate them against the current workflow surface before publishing new release media. **This repo’s CI** uses **`--workflow test`** (**`go vet`** in Docker only; **govulncheck**/**gosec** on the host). The **long** variant also runs **`dockpipe --version`** first.

**Historical recording:** the existing casts show the old `test-demo` review flow. Treat them as archived material until the demo script is refreshed for the current workflows.

## One command (builds both GIFs)

From the repo root (or from `src/scripts/` — same Makefile forwards to the root):

```bash
make demo-record
```

Output:

- `release/demo/dockpipe-demo-short.gif` — compact layout (quick social / thumbnail)
- `release/demo/dockpipe-demo-long.gif` — wider terminal + version + workflow (longer story)
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
bash src/scripts/record-demo.sh all    # or: short | long
```
