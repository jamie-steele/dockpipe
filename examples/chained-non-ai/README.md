# Chained workflow (non-AI)

A full example: run **lint → test → build** (or **generate → validate**) in separate containers. Each step runs in a fresh environment; the same directory is mounted so the next step sees the previous step’s output.

Run from the **dockpipe repo root**.

---

## Prerequisites

- Docker
- dockpipe on PATH (or use `./bin/dockpipe` from repo root)

---

## Option A: Lint → test → build

This directory contains a minimal “project”: a Makefile and a small script. Each step runs in its own container.

```bash
# From dockpipe repo root
WORKDIR="$(pwd)/examples/chained-non-ai"

dockpipe --workdir "$WORKDIR" -- make lint && \
dockpipe --workdir "$WORKDIR" -- make test && \
dockpipe --workdir "$WORKDIR" -- make build
```

- **lint** — Checks `src/script.sh` exists and runs shellcheck if available.
- **test** — Runs a trivial “test” (ensures build artifact or script is present).
- **build** — Writes `build/out.txt` (simulated build artifact).

After the chain, `examples/chained-non-ai/build/out.txt` exists. Each step saw the same `/work` (this directory) but ran in an isolated container.

---

## Option B: Pipe output between containers

Generate a config on the host (or in one container), then validate it in another container:

```bash
# From dockpipe repo root
WORKDIR="$(pwd)/examples/chained-non-ai"

dockpipe --workdir "$WORKDIR" -- ./scripts/generate-config.sh \
  | dockpipe --workdir "$WORKDIR" -- ./scripts/validate-config.sh
```

- **generate-config.sh** — Outputs a simple config to stdout.
- **validate-config.sh** — Reads config from stdin and checks it (exits 0 if valid).

You can replace these with your own generators/validators; the pattern is the same.

---

## Layout

```
chained-non-ai/
├── README.md           # This file
├── Makefile            # Targets: lint, test, build
├── src/
│   └── script.sh       # Sample script to lint/test
├── scripts/
│   ├── generate-config.sh
│   └── validate-config.sh
└── build/              # Created by make build (gitignored or empty)
```

Use this as a template: point `--workdir` at your own project and run your own commands in sequence.
