# Onboarding

**Prerequisites:** [Docker](https://docs.docker.com/get-docker/) and **bash** — see **[install.md](install.md)**.

---

## 1. First run

```bash
dockpipe -- pwd
```

If something fails, **`dockpipe doctor`** checks **bash**, **Docker**, and bundled assets.

---

## 2. Primitive: run → isolate → act

| Phase | Where | What |
|--------|--------|------|
| **Run** | Host | Optional scripts before the container (`run:` / `--run`). |
| **Isolate** | Container | Your command after **`--`**; project at **`/work`**. |
| **Act** | Host or container | Optional script after the main command (see **[architecture.md](architecture.md)**). |

Most days: **`dockpipe -- <command>`** only.

---

## 3. Try a workflow

```bash
dockpipe --workflow test --runtime docker
```

- **`--workflow test`** — **This repo’s CI** uses **`go vet`** in Docker only (no `go test`); **govulncheck** / **gosec** run on the **host** in the same job.  
- **`--workflow test-demo`** — **Recording**: **`go test`** → **`go vet`** → **review prep bundle** → **local-summary** (**`isolate: ollama`**, dockpipe-built **`dockpipe-ollama`**) → **Codex** final review (`make demo-record`; needs **`OPENAI_API_KEY`** for the last step). Prep scripts: **`templates/core/bundles/review-pipeline/`** (workflow asset pack — not a resolver).

Mount **`--mount "$(go env GOPATH)/pkg:/go/pkg:rw"`** so module data is visible in the container. Workflows load from the **materialized bundle** or **`shipyard/workflows/`** / **`templates/`** in a checkout.

To reuse **`shipyard/workflows/`** presets in another tree, copy the directory or use **`dockpipe init`** with **`--from`** pointing at that path (see **[AGENTS.md](../AGENTS.md)**).

---

## 4. Concepts (same words everywhere)

| Term | Meaning |
|------|---------|
| **Workflow** | What happens — **`config.yml`**, **`--workflow <name>`**. |
| **Runtime** | Where execution runs — **`templates/core/runtimes/<name>`** (or **`shipyard/core/runtimes/`** in the cache). |
| **Resolver** | Which tool or platform — **`templates/core/resolvers/<name>`** (or **`shipyard/core/resolvers/`**). |
| **Strategy** | Lifecycle wrapper — **`templates/core/strategies/<name>`**, optional **`strategy:`** in YAML. |
| **Assets** | Support files — **`templates/core/assets/`** (`scripts/`, `images/`, `compose/`). |

Details: **[architecture-model.md](architecture-model.md)** · **[isolation-layer.md](isolation-layer.md)**.

---

## 5. Next steps

| Doc | Use when |
|-----|----------|
| [workflow-yaml.md](workflow-yaml.md) | Editing **`config.yml`**, **`steps:`**, strategies |
| [cli-reference.md](cli-reference.md) | Flags and precedence |
| [chaining.md](chaining.md) | Multiple **`dockpipe`** runs, same workdir |
| [wsl-windows.md](wsl-windows.md) | Optional WSL bridge on Windows |
