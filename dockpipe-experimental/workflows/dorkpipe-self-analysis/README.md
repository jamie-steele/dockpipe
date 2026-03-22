# dorkpipe-self-analysis

**DockPipe** workflow that runs **DorkPipe** over the **mounted repository** in an **isolated container** (`golang:1.25-bookworm` at **`/work`**). That matches DockPipe’s core model: **work runs in a container**, not on the host.

| Output | Purpose |
|--------|---------|
| `.dockpipe/orchestrator-cursor-prompt.md` | Full handoff (sections 1–9) |
| **`.dockpipe/paste-this-prompt.txt`** | **Single block to paste into an AI assistant** — also **printed to stdout** at end of `run-self-analysis.sh` |
| `.dorkpipe/self-analysis/*.txt` | Raw facts (git, package counts, ripgrep hits) — auditable |
| `.dockpipe/orchestrator-cursor-prompt.refined.md` | Only with **`spec.combined.yaml`**: Ollama refine; merged into `paste-this-prompt.txt` |

The workflow **`cmd`** runs in **`golang:1.25-bookworm`** (git and curl from the image; no **`apt-get`** — DockPipe runs the container as your host uid, so package installs as root are not available). It builds **`bin/dorkpipe`** inside the container if missing, then runs **`scripts/dorkpipe/run-self-analysis.sh`** (signals use **`rg`** when present, else **`grep`**).

**Full YAML lifecycle (Compose up → analysis → Compose down):** use **`dorkpipe-self-analysis-stack`** — see **`../dorkpipe-self-analysis-stack/README.md`**.

## Run (default — Docker + isolation)

```bash
# From repo root — use the repo launcher (not bare `dockpipe` unless installed on PATH)
make build
./bin/dockpipe --workflow dorkpipe-self-analysis --workdir . --
```

Direct script (still uses **host** — no container):

```bash
make build
./scripts/dorkpipe/run-self-analysis.sh
```

## Local sidecar stack (Ollama + Postgres) — optional

Bring up **long-lived** services for DAG nodes that need **`OLLAMA_HOST`** / **`DATABASE_URL`**. Tear down when finished — nothing stays running unless you want it.

Postgres is mapped to **host port `15432`** (not `5432`) so it does not fight a system Postgres on the default port.

```bash
scripts/dorkpipe/dev-stack.sh up    # postgres + ollama
scripts/dorkpipe/dev-stack.sh ps
scripts/dorkpipe/dev-stack.sh down
```

Compose file: **`templates/core/bundles/dorkpipe/assets/compose/docker-compose.yml`**. Example DSN: **`postgresql://dorkpipe:dorkpipe@127.0.0.1:15432/dorkpipe`**.

## Host-only workflow (no Docker)

Use **`dorkpipe-self-analysis-host`** when Docker isn’t available or you want the fastest iteration on the host:

```bash
./bin/dockpipe --workflow dorkpipe-self-analysis-host --workdir . --
```

## Combined spec (Ollama refine inside DorkPipe)

**`spec.combined.yaml`** adds an **Ollama** node. From the **host**, point **`OLLAMA_HOST`** at a running Ollama (e.g. after **`dev-stack.sh up`** or **`ollama serve`**).

Running **`spec.combined.yaml` via the containerized DockPipe workflow** may need **`OLLAMA_HOST`** to reach the **host** (not `127.0.0.1` from inside the isolate). Typical fixes: set **`OLLAMA_HOST=http://host.docker.internal:11434`** (Docker Desktop) or **`http://172.17.0.1:11434`** (Linux bridge), or run **`DORKPIPE_SELF_ANALYSIS_SPEC=.../spec.combined.yaml ./scripts/dorkpipe/run-self-analysis.sh`** on the **host** after **`make build`**.

```bash
DORKPIPE_SELF_ANALYSIS_SPEC=dockpipe-experimental/workflows/dorkpipe-self-analysis/spec.combined.yaml \
  ./scripts/dorkpipe/run-self-analysis.sh
```

## Requirements

- **Default workflow:** Docker, **`golang:1.25-bookworm`** pull
- **Direct script:** `make build` → **`bin/dorkpipe`**, `bash`, `git`, `find`, `wc`, **`rg`** recommended

## Principles

- **No fake analysis**: prep/signals only record command output.
- **DockPipe** is the fabric; **DorkPipe** is the DAG orchestrator on top.

See **`docs/dorkpipe.md`** and **`AGENTS.md`**.
