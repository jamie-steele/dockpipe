# Pipeon (product line)

**Pipeon** is separate from the DockPipe primitive: local-first assistant UI, **Code OSS fork** story, VS Code extension, and the **`src/bin/pipeon`** harness (repo root). Everything for that lives under **`src/apps/pipeon/`** so top-level **`docs/`** stays focused on DockPipe / DorkPipe / workflow contracts.

Optional VS Code / Cursor tasks: merge **`vscode-tasks.json.example`** into your workspace **`.vscode/tasks.json`** if you want Pipeon menu entries (the repo root **`.vscode`** is intentionally minimal).

| Location | What |
|----------|------|
| **[docs/](docs/)** | Pipeon IDE experience, VS Code fork playbook, architecture, shortcuts, keybindings example |
| **[scripts/](scripts/)** | Harness entrypoints (symlinks into **`.staging/packages/dockpipe/ide/pipeon/`**), icon generation, desktop shortcuts, code-server launch helpers |
| **[../apps/pipeon-launcher/](../apps/pipeon-launcher/)** | **Pipeon Launcher** (Qt tray app) |
| **[../../src/contrib/pipeon-vscode-extension/](../../src/contrib/pipeon-vscode-extension/)** | **Pipeon IDE** VS Code extension |

Bounded context and how this sits next to DockPipe: **[../../docs/core-tools.md](../../docs/core-tools.md)**.
