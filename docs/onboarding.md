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
dockpipe --workflow test --runtime dockerimage
```

- **`--workflow test`** — **This repo’s CI** uses **`go vet`** in Docker only (no `go test`); **govulncheck** / **gosec** run on the **host** in the same job.  
- **`--workflow test-demo`** — **Recording**: **`go test`** → **`go vet`** → **review prep bundle** → **local-summary** (**`isolate: ollama`**, dockpipe-built **`dockpipe-ollama`**) → **Codex** final review (`make demo-record`; needs **`OPENAI_API_KEY`** for the last step). Prep scripts: **`workflows/review-pipeline/`** in this repo (referenced as **`scripts/review-pipeline/…`** — not a resolver).

Mount **`--mount "$(go env GOPATH)/pkg:/go/pkg:rw"`** so module data is visible in the container. Workflows load from the **materialized bundle** or **`workflows/`** / **`templates/`** in a checkout.

To reuse **`workflows/`** presets in another tree, copy the directory or use **`dockpipe init`** with **`--from`** pointing at that path (see **[AGENTS.md](../AGENTS.md)**).

---

## 4. Concepts (same words everywhere)

| Term | Meaning |
|------|---------|
| **Workflow** | What happens — **`config.yml`**, **`--workflow <name>`**. |
| **Runtime** | **Core** concept — **where** execution runs: profiles under **`templates/core/runtimes/<name>`** (or **`bundle/core/runtimes/`** in the cache). Top-level `runtime` sets the workflow default; a step can override it. |
| **Resolver** | Which tool or platform — **`templates/core/resolvers/<name>`** (or **`bundle/core/resolvers/`**). Top-level `resolver` sets the workflow default; a step can override it. |
| **Strategy** | Lifecycle wrapper — **`templates/core/strategies/<name>`**, optional **`strategy:`** in YAML. |
| **Assets** | Support files — **`templates/core/assets/`** (`scripts/`, `images/`, `compose/`). |

If you are authoring workflow YAML, the normal path is:

1. use **`steps:`**
2. set **`runtime`** + **`resolver`** at the top
3. override them on a step only when that step genuinely differs
4. add **`security`** when the workflow needs to declare network/filesystem/process policy
5. use **`isolate`** only when you must pin a specific image/template
6. treat top-level **`run`** / **`act`** as compact single-flow shorthand only, not step-workflow defaults

Details: **[architecture-model.md](architecture-model.md)** · **[isolation-layer.md](isolation-layer.md)**.

---

## 5. Next steps

| Doc | Use when |
|-----|----------|
| [workflow-yaml.md](workflow-yaml.md) | Editing **`config.yml`**, **`steps:`**, **`resolver`**, **`strategy`**, **`runtime`** |
| [workflow-authoring.md](workflow-authoring.md) | Short workflow authoring path before the full reference |
| [package-quickstart.md](package-quickstart.md) | Compile/package/reuse flow |
| [security-policy.md](security-policy.md) | Container security profiles and effective policy |
| [image-artifacts.md](image-artifacts.md) | Docker image build/reuse artifacts |
| [package-model.md](package-model.md) | Authoring vs packages, **`compile.*`** in **`dockpipe.config.json`**, how workflows relate to resolver/runtime/strategy slices |
| [cli-reference.md](cli-reference.md) | Flags and precedence |
| [workflow-yaml.md](workflow-yaml.md) § Chaining | Multiple **`dockpipe`** runs, same workdir |
| [wsl-windows.md](wsl-windows.md) | Optional WSL bridge on Windows |
