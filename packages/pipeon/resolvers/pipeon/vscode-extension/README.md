# DorkPipe — VS Code extension (minimal)

Install this into **stock VS Code** for development, or pack as **`.vsix`** and install into your **Pipeon** app/editor surface. The outer product can still be Pipeon while the in-editor assistant surface is **DorkPipe**.

## Commands

| Command | Action |
|---------|--------|
| **DorkPipe: Open context bundle** | Shows **`bin/.dockpipe/pipeon-context.md`** in an output channel (run `packages/pipeon/resolvers/pipeon/bin/pipeon bundle` in the repo first). |
| **DorkPipe: Open docs** | Opens the extension doc from the **`pipeon`** resolver when the workspace is this repository. |

The **`images/`** directory holds both the Pipeon app mark and the DorkPipe extension mark. The extension now uses **`dorkpipe-icon.png`** for its Marketplace/activity-bar identity, while Pipeon browser/app assets continue to use the Pipeon mark.

**`dockpipe-code-server:latest`** (Coder code-server in the browser) is built via **`Dockerfile.code-server`** in this directory; **`code-server-user-settings.json`** is the default User settings baked into that image. The image installs the **DorkPipe** extension plus **DockPipe Language Support**. **`make build-code-server-image`** runs the Docker build from the repo root.

## Pack

```bash
make package-pipeon-vscode-extension
```

This writes a VSIX to:
`bin/.dockpipe/extensions/pipeon-<version>.vsix`

Install the generated `.vsix`: **Extensions → … → Install from VSIX…**

Or install via CLI:

```bash
make install-pipeon-vscode-extension
```

## Develop

Open this folder in VS Code and press **F5** (Extension Development Host).

## Note

This is still a **stub**: the richer chat/orchestration story lives in the Pipeon app/backend stack, while this extension provides the in-editor DorkPipe surface.
