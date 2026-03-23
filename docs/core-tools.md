# Core tools & boundaries

This repo contains **three** product-shaped areas plus shared templates/scripts. They are **separate bounded contexts**: different entrypoints, dependencies, and ship paths. Integration is **explicit** (subprocess, files, env) — not shared Go imports between “engines.”

## Map

| Tool | Location | Responsibility |
|------|----------|------------------|
| **DockPipe** | `src/cmd/dockpipe/`, `src/lib/dockpipe/` | **Primitive:** spawn container or host step → run command → optional action. Workflow YAML, Docker, bash pre/act. |
| **DorkPipe** | `src/cmd/dorkpipe/`, `src/lib/dorkpipe/` | **Orchestration / AI reasoning:** DAG specs, workers, escalation — **uses** DockPipe as the execution primitive where needed. |
| **Pipeon Launcher** | `src/apps/pipeon-launcher/` | **Native UI:** saved contexts, launches **`dockpipe`** as a child process, logs, tray. |
| **Pipeon IDE** | `src/contrib/pipeon-vscode-extension/` | **Editor UI:** VS Code extension; talks to the workspace and user — not a second copy of DockPipe’s engine. |

**Pipeon docs + harness** (IDE experience, fork playbook, **`src/bin/pipeon`**) live under **`src/pipeon/`** (see **`src/pipeon/README.md`**). The main **`docs/`** index links via **`docs/pipeon.md`**. Optional VS Code tasks: **`src/pipeon/vscode-tasks.json.example`**.

**Experimental / maintainer workflows** for this repo live under **`shipyard/workflows/`**. User-facing scaffolds stay under **`templates/`**.

### DorkPipe scripts — canonical bundle + repo view

**DorkPipe does not require** shell to live under `scripts/dorkpipe/`. The **`dorkpipe`** binary reads a DAG YAML and runs whatever **commands** the spec lists (paths are just strings).

Workflow resolution (`src/lib/dockpipe/infrastructure/paths.go`): paths under **`scripts/…`** use the project’s **`scripts/`** if present, else **`templates/core/resolvers/…`** (resolver-owned host scripts), else **`templates/core/bundles/…`** (domain asset packs: **`dorkpipe/`**, **`pipeon/`**, **`review-pipeline/`**, …), else **`templates/core/assets/scripts/…`** (agnostic root only).

| Location | Role |
|----------|------|
| **`templates/core/bundles/dorkpipe/`** | **Canonical** bundled DorkPipe assets (DAG helpers, prompts, user-insight queue, compliance handoff). Domain docs under **`bundles/dorkpipe/assets/docs/`** (not under generic **`templates/core/assets/`**). |
| **`src/scripts/dorkpipe/`** | **Maintainer-only** scripts (self-analysis, CI normalize, dev-stack, etc.). Bundled DorkPipe scripts live under **`templates/core/bundles/dorkpipe/assets/scripts/`**; YAML still uses **`scripts/dorkpipe/…`** (resolved by `paths.go`). |

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
- Pipeon docs & harness: [src/pipeon/README.md](../src/pipeon/README.md)
