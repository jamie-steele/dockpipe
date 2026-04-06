# Pipeon — VS Code extension (minimal)

Install this into **stock VS Code** for development, or pack as **`.vsix`** and install into your **Pipeon Code OSS fork** (longer fork notes: **`pipeon-vscode-fork.md`** in the **`pipeon`** resolver **`assets/docs/`**, or this directory for build/pack steps).

## Commands

| Command | Action |
|---------|--------|
| **Pipeon: Open context bundle** | Shows **`.dockpipe/pipeon-context.md`** in an output channel (run `packages/pipeon/resolvers/pipeon/bin/pipeon bundle` in the repo first). |
| **Pipeon: Open fork & extension docs** | Opens the fork doc from the **`pipeon`** resolver **`assets/docs/`** when the workspace is this repository. |

The **`images/`** directory holds the Pipeon **P** mark: **`icon.png`** (128×128), **`favicon.ico`**, **`favicon.svg`**, **`favicon-dark-support.svg`** (browser tab + desktop shortcuts). Regenerate from repo root: **`make pipeon-icons`** (needs **Pillow**).

**`dockpipe-code-server:latest`** (Coder code-server in the browser) is built via **`Dockerfile.code-server`** in this directory; **`code-server-user-settings.json`** is the default User settings baked into that image. **`make build-code-server-image`** runs the Docker build from the repo root.

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

This is a **stub**: chat UI, Ollama, and the Pipeon worker are integrated in the **forked editor** + backend, not only here.
