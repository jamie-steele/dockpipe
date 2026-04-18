# Pipeon — architecture (gateway, MCP, editor shell)

Pipeon is a **Cursor / VS Code–class** product: **editor + workspace + chat**, backed by **DorkPipe over MCP** for orchestration and local inference, with **cloud accounts** available later as optional backends.

The **editor shell** is implemented by **forking VS Code (Code OSS)** and layering Pipeon-specific UI and the worker—see **`pipeon-vscode-fork.md`**.

This **dockpipe** repository ships **contracts** (artifacts under **`.dockpipe/`** / **`bin/.dockpipe/packages/dorkpipe/`**), a **shell harness** (`packages/pipeon/resolvers/pipeon/bin/pipeon`), and a **VS Code extension** stub under **`packages/pipeon/resolvers/pipeon/vscode-extension/`** that you install into your fork or stock VS Code.

---

## Layers

| Layer | Role |
|--------|------|
| **Client** | VS Code–compatible shell (your **fork** of Code OSS, branded Pipeon). |
| **Pipeon extension** | In-IDE commands, panels, chat UI, and attachment picking; talks to a local Pipeon MCP proxy, which forwards into DorkPipe MCP. |
| **DorkPipe control plane** | Aggregates artifacts, runs DockPipe/Docker when needed, routes inference, and owns the MCP boundary. |
| **Internal model service** | Default local inference inside the isolated DorkPipe stack. |
| **Cloud (later)** | Optional accounts; same UX. |

---

## See also

- **`pipeon-ide-experience.md`** — UX and tone  
- **`pipeon-vscode-fork.md`** — how to fork VS Code and wire Pipeon  
