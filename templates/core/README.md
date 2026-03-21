# Shared template assets (`templates/core/`)

Bundled **scaffolding** copied by **`dockpipe init`**. **Architecture** is defined in **[docs/architecture-model.md](../../docs/architecture-model.md)** — not inferred from folder names alone.

**Workflow shapes** (user-facing **`--workflow <name>`**) live under **`templates/<name>/`** at the repo root — **`init`**, **`run`**, **`test`**, **`run-apply-validate`**, … — not under **`core/`**. **Strategies** (e.g. **`worktree`**, **`commit`**) are **`KEY=value`** files here, not duplicate workflow trees.

| Subfolder | Role |
|-----------|------|
| **`runtimes/`** | **Runtime** substrates — **where** execution runs (**`cli`**, **`docker`**, **`kube-pod`**; **`DOCKPIPE_RUNTIME_*`**). |
| **`resolvers/`** | **Resolver** profiles — **which** tool/platform (**`profile`** or flat file **`DOCKPIPE_RESOLVER_*`**, optional **`config.yml`** delegate). |
| **`strategies/`** | Lifecycle strategy files (**`worktree`**, **`commit`**, …). |

The runner **merges** `runtimes/<name>` with `resolvers/<name>` when both exist, or you can set **`--runtime`** and **`--resolver`** to different basenames.

**Resolution:** runtime **`templates/core/runtimes/<name>`** (file or **`profile`**); resolver **`templates/core/resolvers/<name>`** (file or **`profile`**) → legacy **`templates/run-worktree/resolvers/<name>`**. **Strategies:** **`templates/<workflow>/strategies/<name>`** → **`templates/core/strategies/<name>`** → legacy **`templates/strategies/<name>`**.

**`dockpipe init`** copies **`templates/core/`** into your workspace.
