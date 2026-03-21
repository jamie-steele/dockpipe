# Shared resolvers (`templates/core/resolvers/`)

Each file is **`KEY=value`** lines. The Go runner reads **`DOCKPIPE_RESOLVER_*`** only; other lines are comments (`#`).

## Contract

| Key | Required | Meaning |
|-----|----------|---------|
| **`DOCKPIPE_RESOLVER_TEMPLATE`** | Usually yes | Built-in template name passed to **`TemplateBuild`** → Docker image (e.g. `claude`, `codex`, `vscode`, `base-dev`). **Omit** when **`DOCKPIPE_RESOLVER_WORKFLOW`** or **`DOCKPIPE_RESOLVER_HOST_ISOLATE`** is set. |
| **`DOCKPIPE_RESOLVER_WORKFLOW`** | no | Bundled workflow under **`templates/<name>/config.yml`** (e.g. `claude`, `codex`, `code-server`, `cursor-dev`, `vscode`). After **`run`** pre-scripts, the runner executes that workflow with the **same** engine as **`dockpipe --workflow <name>`**. **Mutually exclusive** with **`DOCKPIPE_RESOLVER_HOST_ISOLATE`**. |
| **`DOCKPIPE_RESOLVER_HOST_ISOLATE`** | no | Repo-relative script run on the **host** after pre-scripts instead of **`docker run`**. Use when there is no bundled workflow; prefer **`DOCKPIPE_RESOLVER_WORKFLOW`** when **`templates/<name>`** already exists. |
| **`DOCKPIPE_RESOLVER_PRE_SCRIPT`** | no | Host script when using **`--resolver`** *without* **`--workflow`** (defaults from workflow **`run`** otherwise). |
| **`DOCKPIPE_RESOLVER_ACTION`** | no | Act script for resolver-only runs; **`--workflow`** uses **`config.yml`** **`act`**. |
| **`DOCKPIPE_RESOLVER_CMD`** | no | Default CLI name for documentation; **not** executed by dockpipe. |
| **`DOCKPIPE_RESOLVER_ENV`** | no | Comma-separated env var names you usually need (documentation). |
| **`DOCKPIPE_RESOLVER_EXPERIMENTAL`** | no | Set to **`1`** to print an experimental warning on stderr. |

**Lifecycle:** **`run`** (clone/worktree) → **isolate** (container from **`DOCKPIPE_RESOLVER_TEMPLATE`**, **or** embedded workflow from **`DOCKPIPE_RESOLVER_WORKFLOW`**, **or** host script from **`DOCKPIPE_RESOLVER_HOST_ISOLATE`**) → **`act`** (commit on host when using bundled **`commit-worktree.sh`**).

Adding a backend = add **`resolvers/<name>`** + list it under **`resolvers:`** in **`config.yml`**. No conditionals in **`config.yml`**.

**Multi-step workflows** can set **`resolver: <name>`** on an individual **`steps:`** entry so that step loads the same **`resolvers/<name>`** file (template, host isolate, default act). **`isolate:`** on that step overrides **`DOCKPIPE_RESOLVER_TEMPLATE`** when you need an explicit image. Async steps (`is_blocking: false`) cannot use resolvers that define **`DOCKPIPE_RESOLVER_HOST_ISOLATE`**.
