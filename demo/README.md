# Demo recording (developer / social)

Record a short terminal clip of:

`dockpipe --workflow test --runtime docker`

## One command

From the repo root:

```bash
make demo-record
```

Output:

- `demo/dockpipe-demo.gif` — shareable loop
- `demo/dockpipe-demo.cast` — asciinema source (can delete after rendering)

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
bash scripts/record-demo.sh
```
