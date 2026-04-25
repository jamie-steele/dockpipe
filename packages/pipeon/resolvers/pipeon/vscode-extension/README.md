# DorkPipe — VS Code extension

Install this into **stock VS Code** for development, or pack as **`.vsix`** and install into your **Pipeon** app/editor surface. The outer product can still be Pipeon while the in-editor assistant surface is **DorkPipe**.

## What it does now

- Persistent per-workspace chat history with multiple chat sessions
- **New chat** and **clear current chat** commands
- Natural-language requests are first routed by DorkPipe into `chat`, `inspect`, or `edit`
- Explicit **`/edit ...`** requests still work as an override, but they are not the main UX
- Safe local orchestration for obvious actions before the model call
- Edit flows prepare and validate a patch first, then ask for confirmation before applying
- Streaming status breadcrumbs before and during the DorkPipe request
- Markdown-style assistant rendering for headings, lists, code fences, block quotes, and inline code
- Workspace-aware prompts built from the current context bundle, active file, selection, and recent chat turns
- Workspace file attachments routed through DorkPipe for read-only context enrichment
- MCP-only request/apply flow through a local Pipeon MCP proxy; the extension refuses to execute DorkPipe locally and requires only **`MCP_HTTP_URL`**

## Commands

| Command | Action |
|---------|--------|
| **DorkPipe: Open chat** | Reveal the DorkPipe chat panel. |
| **DorkPipe: New chat** | Start a fresh persisted chat session in the current workspace. |
| **DorkPipe: Clear current chat** | Clear the active session without deleting other saved chats. |
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

The extension source now lives in **`src/`** as TypeScript and compiles back into the runtime entrypoints under this package.

Source of truth:

- Edit **`src/extension.ts`** and **`src/webview/*.ts`**
- Treat **`extension.js`** and **`webview/*.js`** as generated runtime artifacts
- Keep generated DockPipe runtime/cache trees such as **`.dockpipe/`** and **`bin/.dockpipe/`** out of normal editor workflows; the baked Pipeon user settings exclude them so stale compiled package snapshots do not bleed into Problems or search

Build the runtime files after edits:

```bash
npm run build
```

Run the package-local TypeScript checks before packaging or larger refactors:

```bash
npm run typecheck
```

## Note

This extension now carries a meaningful in-editor chat surface, but the long-term product boundary is still the same: Pipeon owns UX, DorkPipe is the server-authoritative orchestration layer, and DockPipe remains the mutation boundary.

Attachment roadmap:

- `File` upload is wired for local workspace files
- `Image` upload is scaffolded but still TODO
- `PDF` upload is scaffolded but still TODO
