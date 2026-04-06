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

## Quick start

```bash
export DOCKPIPE_PIPEON=1
export DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1   # until VERSION >= min gate
./packages/pipeon/resolvers/pipeon/bin/pipeon status
./packages/pipeon/resolvers/pipeon/bin/pipeon bundle
./packages/pipeon/resolvers/pipeon/bin/pipeon chat "Summarize security posture from available signals."
```

See **`assets/docs/pipeon-ide-experience.md`** and **`assets/docs/pipeon-shortcuts.md`**.

## Pipeon ↔ DorkPipe boundary

The internal client/orchestrator contract lives in **`assets/docs/pipeon-dorkpipe-contract.md`**. Pipeon is
the chat client and UX shell; DorkPipe remains server-authoritative for routing and validation; DockPipe is
the isolated mutation boundary.

## VS Code extension and fork

The checked-in extension (browser tab, context command) lives under **`vscode-extension/`** next to this resolver — see **`vscode-extension/README.md`** for packaging and **code-server** image build.

## Qt launcher

The native tray app is **`src/apps/pipeon-launcher/`** (separate binary; not this resolver).
