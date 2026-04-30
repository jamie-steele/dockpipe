# Core tools (this repository)

| Area | Code / doc |
|------|------------|
| **DockPipe** | **`src/cmd/`**, **`src/lib/`** |
| **DorkPipe** | Maintainer **`dorkpipe`** pack — **`dorkpipe/lib/README.md`** |
| **Pipeon** | First-party **`packages/pipeon/`** — **`packages/pipeon/resolvers/pipeon/README.md`** (VS Code extension under **`vscode-extension/`**) |
| **MCP** | **DorkPipe-owned MCP bridge** — **`packages/dorkpipe/mcp/README.md`** |

**Package index:** [`.staging/packages/README.md`](../.staging/packages/README.md).

Recommended contributor / package-author loop for this repository:

```bash
make build
make dev-install
dockpipe package build
```

- **`make build`** builds the DockPipe CLI and DockPipe Launcher for this checkout.
- **`make dev-install`** is a repo-contributor convenience: it installs the freshly built `dockpipe` into your local user PATH so plain `dockpipe ...` resolves to the new build.
- **`dockpipe package build`** runs package-owned source builds for packages that declare **`build.source.script`** in **`package.yml`**.
- **`dockpipe package test`** runs package-owned tests for packages that declare **`test.script`** in **`package.yml`**.

Script paths like **`scripts/dorkpipe/…`** resolve per **`src/lib/infrastructure/paths.go`** (project `scripts/` → compiled packages → config compile roots → templates).
