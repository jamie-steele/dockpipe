# Pipeon resolver (IDE pack)

**Pipeon** is an **IDE-oriented resolver** in the **`ide`** package: same layout as **`vscode`** and **`cursor-dev`** (`profile/`, **`config.yml`**, **`assets/`**). It is the **product harness** for a local-first assistant (Ollama chat, **`.dockpipe/pipeon-context.md`** bundle) — not an agent-style model resolver under **`agent/`**. Pipeon owns the browser/editor lane directly; there is no separate standalone `code-server` product surface in this repo.

Package intent: the **`pipeon`** package should travel with **`dorkpipe`** and **`dorkpipe-mcp`** for the
full local assistant stack. Its package manifest declares those as **`depends`** so compile/store/release
flows can treat Pipeon as the top-level product surface rather than a disconnected IDE skin.

## Layout

| Path | Purpose |
|------|---------|
| **`profile`** | Resolver label for packaging next to other IDE resolvers. |
| **`config.yml`** | Workflow metadata for the **`pipeon`** leaf (namespace + pack wiring). |
| **`assets/scripts/`** | **`pipeon.sh`**, **`chat.sh`**, **`bundle-context.sh`**, installers, **`generate-pipeon-icons.py`**. |
| **`assets/docs/`** | Pipeon UX, shortcuts, architecture notes. |
| **`vscode-tasks.json.example`** | Optional VS Code / Cursor tasks (copy into User or workspace). |
| **`../pipeon-dev-stack/`** | First-party local product stack: Pipeon UI + DorkPipe sidecars + MCP bridge. |

**Entrypoint on PATH:** **`packages/pipeon/resolvers/pipeon/bin/pipeon`** in this repository runs **`assets/scripts/pipeon.sh`**.

## Authoring note

When adding new Pipeon scripts, prefer the shared core SDK:

- **Shell:** use **`dockpipe get ...`** for plain context reads; bootstrap **`eval "$(dockpipe sdk)"`** only for shell-specific actions like **`dockpipe_sdk init-script`**
- **`src/core/assets/scripts/lib/repo-tools.ps1`**
- **`src/core/assets/scripts/lib/repo_tools.py`**
- **`src/core/assets/scripts/lib/repotools/repotools.go`**

That shared SDK surface resolves the real repo-local **`dockpipe`** build first (for example **`src/bin/dockpipe`**) before falling back to `PATH`, so maintainer/dev flows do not silently depend on a stale global install.

## Quick start

```bash
export DOCKPIPE_PIPEON=1
export DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1   # until VERSION >= min gate
pipeon status
pipeon bundle
pipeon chat "Summarize security posture from available signals."
```

See **`assets/docs/pipeon-ide-experience.md`**, **`assets/docs/pipeon-shortcuts.md`**, and **`assets/docs/pipeon-refresh.md`**.

## Pipeon ↔ DorkPipe boundary

The internal client/orchestrator contract lives in **`assets/docs/pipeon-dorkpipe-contract.md`**. Pipeon is
the chat client and UX shell; DorkPipe remains server-authoritative for routing and validation; DockPipe is
the isolated mutation boundary.

## VS Code extension and fork

The checked-in extension (browser tab, context command) lives under **`vscode-extension/`** next to this resolver — see **`vscode-extension/README.md`** for packaging and **code-server** image build.

## DockPipe Launcher

The native tray app is **`src/app/tooling/dockpipe-launcher/`** (separate binary; not this resolver). Pipeon uses it for first-run setup and app-style launching, but it is DockPipe tooling rather than a Pipeon-owned app.
