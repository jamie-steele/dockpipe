# DorkPipe plugin (`dorkpipe` package)

All DorkPipe **maintainer** workflows for this repo live here under **`resolvers/<name>/`** — one directory per **`--workflow`** leaf ( **`config.yml`** + assets). Umbrella metadata: **`package.yml`**.

| Resolver / workflow | Role |
|---------------------|------|
| **`compliance-handoff/`** | CI + self-analysis signal pointers |
| **`dorkpipe-orchestrator/`** | Example DAG / host orchestrator |
| **`dorkpipe-self-analysis/`** | Container self-analysis |
| **`dorkpipe-self-analysis-host/`** | Host-only variant |
| **`dorkpipe-self-analysis-stack/`** | Compose sidecar + analysis |
| **`dorkpipe/`** | Small domain **`config.yml`** pack (namespace wiring) |
| **`user-insight-process/`** | Host workflow: queue → **`insights.json`** — **`resolvers/user-insight-process/README.md`** |

Go code for the orchestrator lives in **`lib/`** (module **`dorkpipe.orchestrator`**) — this tree is **YAML + assets** only (not the Go module).

**Detail:** **`lib/README.md`** (Go module); this tree is YAML + assets.
