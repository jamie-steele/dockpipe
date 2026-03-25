# `apps/` — desktop & host UIs

Shippable applications that **drive** DockPipe from the host. They do **not** replace the `dockpipe` CLI; they **invoke** it (or match its contract).

| Directory | Tool | Stack |
|-----------|------|-------|
| **[pipeon/](pipeon/)** | **Pipeon** — docs, shell harness (`src/bin/pipeon`), shortcuts, VS Code task example | Bash, Python (icons) |
| **[pipeon-launcher/](pipeon-launcher/)** | **Pipeon Launcher** — tray, contexts, `dockpipe` subprocess | Qt 6, C++ |

**Pipeon IDE** (editor-side integration) lives under **[src/contrib/pipeon-vscode-extension/](../../src/contrib/pipeon-vscode-extension/)** — different runtime (Node/VS Code) than this tree.

Boundaries and how these tools talk to each other: **[../../docs/core-tools.md](../../docs/core-tools.md)**.
