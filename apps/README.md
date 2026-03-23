# `apps/` — desktop & host UIs

Shippable applications that **drive** DockPipe from the host. They do **not** replace the `dockpipe` CLI; they **invoke** it (or match its contract).

| Directory | Tool | Stack |
|-----------|------|--------|
| **[pipeon-launcher/](pipeon-launcher/)** | **Pipeon Launcher** — tray, contexts, `dockpipe` subprocess | Qt 6, C++ |

**Pipeon IDE** (editor-side integration) lives under **[contrib/pipeon-vscode-extension/](../contrib/pipeon-vscode-extension/)**; docs and shell harness live under **[pipeon/](../pipeon/)** — different runtime (Node/VS Code) than this tree.

Boundaries and how these tools talk to each other: **[docs/core-tools.md](../docs/core-tools.md)**.
