# Pipeon — VS Code extension (minimal)

Install this into **stock VS Code** for development, or pack as **`.vsix`** and install into your **Pipeon Code OSS fork** (see **`docs/pipeon-vscode-fork.md`**).

## Commands

| Command | Action |
|---------|--------|
| **Pipeon: Open context bundle** | Shows **`.dockpipe/pipeon-context.md`** in an output channel (run `bin/pipeon bundle` in the repo first). |
| **Pipeon: Open fork & extension docs** | Opens **`docs/pipeon-vscode-fork.md`** when the workspace is this repository. |

The extension lists a **128×128** **`images/icon.png`** (Pipeon **P** on a blue tile). Regenerate all mark assets (PNG, `.ico`, SVG favicons) from repo root: **`make pipeon-icons`** (needs **Pillow**).

## Pack

```bash
npm install
npx --yes @vscode/vsce package
```

Install the generated `.vsix`: **Extensions → … → Install from VSIX…**

## Develop

Open this folder in VS Code and press **F5** (Extension Development Host).

## Note

This is a **stub**: chat UI, Ollama, and the Pipeon worker are integrated in the **forked editor** + backend, not only here.
