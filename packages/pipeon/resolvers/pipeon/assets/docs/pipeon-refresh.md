# Pipeon Build And Refresh

Use this when Pipeon is running against this repository and local code changes are not showing up.

## Which binary does what

- `make build` updates `src/bin/dockpipe`
- `make maintainer-tools` updates `packages/dorkpipe/bin/dorkpipe` and `packages/dorkpipe-mcp/bin/mcpd`
- `npm --prefix packages/pipeon/resolvers/pipeon/vscode-extension run build` updates the checked-in Pipeon extension output
- `make build-pipeon-desktop` updates the Tauri desktop shell at `src/apps/pipeon-desktop/bin/pipeon-desktop`
- `make build-code-server-image` rebuilds the branded Pipeon code-server image with fresh extension assets

## Refresh matrix

### DockPipe CLI only

```bash
make build
```

Use this when you are testing `src/bin/dockpipe` directly.

### DorkPipe / MCP sidecars

```bash
make maintainer-tools
```

Use this when Pipeon Ask/edit behavior depends on `packages/dorkpipe/bin/dorkpipe` or `packages/dorkpipe-mcp/bin/mcpd`.

### Pipeon VS Code extension source

```bash
npm --prefix packages/pipeon/resolvers/pipeon/vscode-extension run build
```

Use this when you changed `packages/pipeon/resolvers/pipeon/vscode-extension/src/`.

### Pipeon desktop shell

```bash
make build-pipeon-desktop
```

Use this when you changed the Tauri desktop host under `src/apps/pipeon-desktop/`.

### Full Pipeon IDE / dev-stack refresh

If you are running Pipeon through the Tauri desktop shell plus the local code-server stack, rebuild all moving parts:

```bash
make build
make maintainer-tools
npm --prefix packages/pipeon/resolvers/pipeon/vscode-extension run build
make build-pipeon-desktop
make build-code-server-image
```

Then fully restart the running surface:

1. Quit Pipeon desktop.
2. Stop the Pipeon dev stack / code-server session.
3. Start it again from the repo root.

Typical restart:

```bash
./src/bin/dockpipe --workflow pipeon-dev-stack --workdir . --
```

## Common gotcha

If Ask mode in Pipeon still shows old behavior after `make build`, you probably rebuilt `dockpipe` but not `dorkpipe`.

Pipeon dev-stack uses:

- `src/bin/dockpipe`
- `packages/dorkpipe/bin/dorkpipe`
- `packages/dorkpipe-mcp/bin/mcpd`

So Ask/edit/runtime changes in `packages/dorkpipe/lib/` need `make maintainer-tools`, not only `make build`.
