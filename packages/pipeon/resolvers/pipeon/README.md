# Pipeon resolver (IDE pack)

**Pipeon** is an **IDE-oriented resolver** in the **`ide`** package: same layout as **`vscode`**, **`cursor-dev`**, and **`code-server`** (`profile/`, **`config.yml`**, **`assets/`**). It is the **product harness** for a local-first assistant (Ollama chat, **`.dockpipe/pipeon-context.md`** bundle) — not an agent-style model resolver under **`agent/`**.

Package intent: the **`pipeon`** package should travel with **`dorkpipe`** and **`dockpipe-mcp`** for the
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

**Entrypoint on PATH:** **`src/bin/pipeon`** in this repository runs **`assets/scripts/pipeon.sh`**.

## Quick start

```bash
export DOCKPIPE_PIPEON=1
export DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1   # until VERSION >= min gate
./src/bin/pipeon status
./src/bin/pipeon bundle
./src/bin/pipeon chat "Summarize security posture from available signals."
```

See **`assets/docs/pipeon-ide-experience.md`** and **`assets/docs/pipeon-shortcuts.md`**.

## VS Code extension and fork

The checked-in extension (browser tab, context command) lives under **`vscode-extension/`** next to this resolver — see **`vscode-extension/README.md`** for packaging and **code-server** image build.

## Qt launcher

The native tray app is **`src/apps/pipeon-launcher/`** (separate binary; not this resolver).
