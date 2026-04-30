# Pipeon Build And Refresh

Use this when the local Pipeon dev stack is running against this repository and code changes are not showing up.

## Which binary does what

- `make build` updates the local `dockpipe` binary and DockPipe Launcher
- `make dev-install` is an optional maintainer convenience that updates your shell `dockpipe` to the freshly built local binary
- `dockpipe package build` runs package-owned source builds for packages in this checkout
- `packages/pipeon/assets/scripts/build.sh vscode-extension` rebuilds the checked-in Pipeon extension output using an external build cache outside `packages/`
- `packages/pipeon/assets/scripts/build.sh desktop` updates the Tauri desktop shell at `packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop` using an external Cargo target dir outside `packages/`
- `packages/pipeon/assets/scripts/build.sh code-server-image` rebuilds the branded Pipeon code-server image with fresh extension assets

## Refresh matrix

### Default local dev loop

```bash
make build
make dev-install
dockpipe package build
```

Use this first. It gives you:

- fresh `dockpipe`
- fresh launcher
- package-owned source builds for first-party packages that need them

`make dev-install` is only for contributors/package authors working from this source checkout. It is not part of the normal end-user/package runtime flow.

### DockPipe CLI only

```bash
make build
```

Use this when you are testing a freshly built local `dockpipe` binary directly.

### DorkPipe / MCP control plane

```bash
dockpipe package build --only dorkpipe
```

Use this when the isolated DorkPipe control plane behavior depends on:

- `packages/dorkpipe/bin/dorkpipe`
- `packages/dorkpipe/bin/mcpd`

### Pipeon VS Code extension source

```bash
packages/pipeon/assets/scripts/build.sh vscode-extension
```

Use this when you changed `packages/pipeon/resolvers/pipeon/vscode-extension/src/`.

### Pipeon desktop shell

```bash
packages/pipeon/assets/scripts/build.sh desktop
```

Use this when you changed the Tauri desktop host under `packages/pipeon/apps/pipeon-desktop/`.

### Full Pipeon IDE / dev-stack refresh

If you are running Pipeon through the Tauri desktop shell plus the local code-server stack, rebuild all moving parts:

```bash
make build
make dev-install
dockpipe package build
packages/pipeon/assets/scripts/build.sh vscode-extension
packages/pipeon/assets/scripts/build.sh desktop
packages/pipeon/assets/scripts/build.sh code-server-image
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

If Pipeon still shows old control-plane behavior after `make build`, you probably rebuilt `dockpipe` but did not rerun the package-owned source builds.

Pipeon dev-stack uses:

- `dockpipe`
- `packages/dorkpipe/bin/dorkpipe`
- `packages/dorkpipe/bin/mcpd`

So DorkPipe request/apply/runtime changes in `packages/dorkpipe/lib/` need `dockpipe package build` (or `dockpipe package build --only dorkpipe`), not only `make build`.
