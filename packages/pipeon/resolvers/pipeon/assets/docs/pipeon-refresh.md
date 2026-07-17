# Pipeon Build And Refresh

Use this when the local Pipeon dev stack is running against this repository and code changes are not showing up.

This guide is for the **Pipeon surface**: the code-server image, VSIX, and local
first-party binaries. It is separate from `pipeon-desktop`, which updates only
the desktop shell.

## Normal behavior

If you launch Pipeon through `pipeon-dev-stack`, the default
`PIPEON_DEV_STACK_BUILD=auto` setting refreshes the branded
`dockpipe-code-server:latest` image when the relevant Pipeon-managed inputs
changed.

That covers the normal user path. Use the manual steps below when you are
working in this repo and need a forced rebuild.

## Manual rebuilds

### Rebuild the Pipeon surface

```bash
packages/pipeon/assets/scripts/build.sh vscode-extension
packages/pipeon/assets/scripts/build.sh code-server-image
```

Use this when you changed Pipeon extension or code-server image inputs and want
an explicit refresh before relaunching the dev stack.

### Rebuild the desktop shell

```bash
packages/pipeon/assets/scripts/build.sh desktop
```

Use this when you changed the Tauri host under
`packages/pipeon/apps/pipeon-desktop/`.

### Rebuild first-party binaries used by the stack

```bash
make build
dockpipe package build --only dorkpipe
```

Use this when the running stack needs fresh local `dockpipe`, `dorkpipe`, or
`mcpd` binaries from this checkout.

## Restart after rebuild

Quit Pipeon desktop, stop the running stack, then launch it again:

```bash
dockpipe --workflow pipeon-dev-stack --workdir . --
```

If Pipeon still shows older control-plane behavior after `make build`, also run
`dockpipe package build --only dorkpipe` so the stack picks up fresh package
binaries from this checkout.
