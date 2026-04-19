# Pipeon Build And Refresh

Use this when the local Pipeon dev stack is running against this repository and code changes are not showing up.

## Which binary does what

- `make build` updates the local `dockpipe` launcher binary
- `make maintainer-tools` updates `packages/dorkpipe/bin/dorkpipe` and `packages/dorkpipe/bin/mcpd`
- `make package-pipeon-vscode-extension` rebuilds the checked-in Pipeon extension output using an external build cache outside `packages/`
- `make build-pipeon-desktop` updates the Tauri desktop shell at `packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop` using an external Cargo target dir outside `packages/`
- `make build-code-server-image` rebuilds the branded Pipeon code-server image with fresh extension assets

## Refresh matrix

### DockPipe CLI only

```bash
make build
```

Use this when you are testing a freshly built local `dockpipe` binary directly.

### DorkPipe / MCP control plane

```bash
make maintainer-tools
```

Use this when the isolated DorkPipe control plane behavior depends on:

- `packages/dorkpipe/bin/dorkpipe`
- `packages/dorkpipe/bin/mcpd`

### Pipeon VS Code extension source

```bash
make package-pipeon-vscode-extension
```

Use this when you changed `packages/pipeon/resolvers/pipeon/vscode-extension/src/`.

### Pipeon desktop shell

```bash
make build-pipeon-desktop
```

Use this when you changed the Tauri desktop host under `packages/pipeon/apps/pipeon-desktop/`.

### Full Pipeon IDE / dev-stack refresh

If you are running Pipeon through the Tauri desktop shell plus the local code-server stack, rebuild all moving parts:

```bash
make build
make maintainer-tools
make package-pipeon-vscode-extension
make build-pipeon-desktop
make build-code-server-image
```

Then fully restart the running surface:

1. Quit Pipeon desktop.
2. Stop the Pipeon dev stack / code-server session.
3. Start it again from the repo root.

Typical restart:

```bash
dockpipe --workflow pipeon-dev-stack --workdir . --
```

## Common gotcha

If Pipeon still shows old control-plane behavior after `make build`, you probably rebuilt `dockpipe` but not the DorkPipe/MCP binaries.

Pipeon dev-stack uses:

- `dockpipe`
- `packages/dorkpipe/bin/dorkpipe`
- `packages/dorkpipe/bin/mcpd`

So DorkPipe request/apply/runtime changes in `packages/dorkpipe/lib/` need `make maintainer-tools`, not only `make build`.
