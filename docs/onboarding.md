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

Two Alpine steps with **`outputs:`** handoff; no API keys. The workflow loads from the **materialized bundle** in your cache (**`dockpipe/workflows/test/`**) or from **`templates/test/`** in a checkout.

To copy presets into **your** repo: **`dockpipe init --dogfood-test`** (see **[cli-reference.md](cli-reference.md)**).

---

## 4. Concepts (same words everywhere)

| Term | Meaning |
|------|---------|
| **Workflow** | What happens — **`config.yml`**, **`--workflow <name>`**. |
| **Runtime** | Where execution runs — **`templates/core/runtimes/<name>`** (or **`dockpipe/core/runtimes/`** in the cache). |
| **Resolver** | Which tool or platform — **`templates/core/resolvers/<name>`** (or **`dockpipe/core/resolvers/`**). |
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
