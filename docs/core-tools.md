# Core tools & boundaries

This repo contains **three** product-shaped areas plus shared templates/scripts. They are **separate bounded contexts**: different entrypoints, dependencies, and ship paths. Integration is **explicit** (subprocess, files, env) — not shared Go imports between “engines.”

## Map

| Tool | Location | Responsibility |
|------|----------|------------------|
| **DockPipe** | `src/cmd/dockpipe/`, `src/lib/dockpipe/` | **Primitive:** spawn container or host step → run command → optional action. Workflow YAML, Docker, bash pre/act. |
| **DorkPipe** | `src/cmd/dorkpipe/`, `src/lib/dorkpipe/` | **Orchestration / AI reasoning:** DAG specs, workers, escalation — **uses** DockPipe as the execution primitive where needed. |
| **Pipeon** (harness + docs) | `src/apps/pipeon/` | Shell harness (**`src/bin/pipeon`**), shortcuts, fork playbook docs; symlinks into **`.staging/bundles/pipeon/`**. |
| **Pipeon Launcher** | `src/apps/pipeon-launcher/` | **Native UI:** saved contexts, launches **`dockpipe`** as a child process, logs, tray. |
| **Pipeon IDE** | `src/contrib/pipeon-vscode-extension/` | **Editor UI:** VS Code extension; talks to the workspace and user — not a second copy of DockPipe’s engine. |

**Pipeon-only docs** (IDE experience, shortcuts): **`src/apps/pipeon/README.md`**, **`docs/pipeon.md`**. Optional VS Code tasks: **`src/apps/pipeon/vscode-tasks.json.example`**.

**Lean CI / dogfood workflows** for this repo live under repo-root **`workflows/`**; **maintainer, packaging, and experiments** live under **`.staging/workflows/`** (still **`--workflow <name>`** — same resolution order as the CLI). User-facing scaffolds stay under **`templates/`**.

### DorkPipe scripts — canonical bundle + repo view

**DorkPipe does not require** shell to live under `scripts/dorkpipe/`. The **`dorkpipe`** binary reads a DAG YAML and runs whatever **commands** the spec lists (paths are just strings).

Workflow resolution (`src/lib/dockpipe/infrastructure/paths.go`): paths under **`scripts/…`** use the project’s **`scripts/`** if present, else **`.staging/resolvers/…`** / **`.staging/bundles/…`** in this repo (merged into **`shipyard/core/…`** when materialized), else lean **`templates/core/resolvers/…`**, else **`templates/core/assets/scripts/…`** (agnostic root only).

| Location | Role |
|----------|------|
| **`.staging/bundles/dorkpipe/`** | **Canonical** DorkPipe asset pack in this repo (DAG helpers, prompts, user-insight queue, compliance handoff). |
| **`src/scripts/dorkpipe/`** | **Maintainer-only** scripts (self-analysis, CI normalize, dev-stack, etc.). YAML still uses **`scripts/dorkpipe/…`** (resolved by `paths.go`). |

Nothing in **`lib/dorkpipe/`** imports these paths — they are **glue**, not Go packages. See [dorkpipe.md](dorkpipe.md#reusable-assets).

## How they communicate (allowed)

- **Subprocess:** Launcher (and scripts) run `dockpipe …` with argv and env — same as any shell.
- **Files:** DorkPipe and DockPipe exchange state via repo files (e.g. `.dockpipe/`, specs, handoff prompts).
- **Env:** `DOCKPIPE_*` and documented flags — no private RPC between Go packages required.

## What to avoid

- **DockPipe `src/lib/dockpipe/`** should not depend on **`src/lib/dorkpipe/`** (keep the runner primitive agnostic).
- **Launcher / VS Code extension** should not embed DockPipe’s Go libraries — they are separate processes/products.
- **DorkPipe** may **invoke** `dockpipe` or share **types** only where already factored (keep imports one-direction: orchestration → primitive).

## Related docs

- DockPipe terms & architecture: [architecture-model.md](architecture-model.md), [architecture.md](architecture.md)
- DorkPipe: [dorkpipe.md](dorkpipe.md)
- Pipeon Launcher build: [src/apps/pipeon-launcher/README.md](../src/apps/pipeon-launcher/README.md)
- Pipeon docs & harness: [src/apps/pipeon/README.md](../src/apps/pipeon/README.md)
