# Core tools (this repository)

| Area | Code / doc |
|------|------------|
| **DockPipe** | **`src/cmd/`**, **`src/lib/`** |
| **DorkPipe** | Maintainer **`dorkpipe`** pack — **`dorkpipe/lib/README.md`** |
| **Pipeon** | First-party **`packages/pipeon/`** — **`packages/pipeon/resolvers/pipeon/README.md`** (VS Code extension under **`vscode-extension/`**) |
| **MCP** | **`dockpipe-mcp`** pack — **`dockpipe-mcp/README.md`** |

**Package index:** [`.staging/packages/README.md`](../.staging/packages/README.md).

**`make build`** emits **`src/bin/dockpipe.bin`** (launcher **`src/bin/dockpipe`**). **`make maintainer-tools`** emits **`dorkpipe`** and **`mcpd`** under those packages’ **`bin/`** dirs.

Script paths like **`scripts/dorkpipe/…`** resolve per **`src/lib/infrastructure/paths.go`** (project `scripts/` → compiled packages → config compile roots → templates).
