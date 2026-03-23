# Pipeon (product line)

**Pipeon** is separate from the DockPipe primitive: local-first assistant UI, **Code OSS fork** story, VS Code extension, and the **`bin/pipeon`** harness. Everything for that lives here so the top-level **`docs/`** tree stays focused on DockPipe / DorkPipe / workflow contracts.

| Location | What |
|----------|------|
| **[docs/](docs/)** | Pipeon IDE experience, VS Code fork playbook, architecture, shortcuts, keybindings example |
| **[scripts/](scripts/)** | Harness entrypoints (symlinks into **`templates/core/bundles/pipeon/`**), icon generation, desktop shortcuts, code-server launch helpers |
| **[../apps/pipeon-launcher/](../apps/pipeon-launcher/)** | **Pipeon Launcher** (Qt tray app) |
| **[../contrib/pipeon-vscode-extension/](../contrib/pipeon-vscode-extension/)** | **Pipeon IDE** VS Code extension |

Bounded context and how this sits next to DockPipe: **[../docs/core-tools.md](../docs/core-tools.md)**.
