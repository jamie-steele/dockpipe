# llm-worktree resolvers

Each file is **`KEY=value`** lines. The Go runner reads **`DOCKPIPE_RESOLVER_*`** only; other lines are comments (`#`).

## Contract

| Key | Required | Meaning |
|-----|----------|---------|
| **`DOCKPIPE_RESOLVER_TEMPLATE`** | yes | Built-in template name passed to **`TemplateBuild`** → Docker image (e.g. `claude`, `codex`, `vscode`, `base-dev`). |
| **`DOCKPIPE_RESOLVER_PRE_SCRIPT`** | no | Host script when using **`--resolver`** *without* **`--workflow`** (defaults from workflow **`run`** otherwise). |
| **`DOCKPIPE_RESOLVER_ACTION`** | no | Act script for resolver-only runs; **`--workflow`** uses **`config.yml`** **`act`**. |
| **`DOCKPIPE_RESOLVER_CMD`** | no | Default CLI name for documentation; **not** executed by dockpipe. |
| **`DOCKPIPE_RESOLVER_ENV`** | no | Comma-separated env var names you usually need (documentation). |
| **`DOCKPIPE_RESOLVER_EXPERIMENTAL`** | no | Set to **`1`** to print an experimental warning on stderr. |

**Lifecycle is unchanged:** **`run`** (clone/worktree) → **isolate** (container from **`DOCKPIPE_RESOLVER_TEMPLATE`**) → **`act`** (commit on host).

Adding a backend = add **`resolvers/<name>`** + list it under **`resolvers:`** in **`config.yml`**. No conditionals in **`config.yml`**.
