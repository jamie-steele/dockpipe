# Pipeon — architecture (gateway, worker, editor shell)

Pipeon is a **Cursor / VS Code–class** product: **editor + workspace + chat**, backed by **Pipeon’s worker** (intelligence + DockPipe/Docker) and **Ollama** on the host by default; **cloud accounts** for paid models later.

The **editor shell** is implemented by **forking VS Code (Code OSS)** and layering Pipeon-specific UI and the worker—see **`pipeon-vscode-fork.md`**.

This **dockpipe** repository ships **contracts** (artifacts under **`.dockpipe/`** / **`.dorkpipe/`**), a **shell harness** (`src/bin/pipeon`), and a **VS Code extension** stub under **`src/contrib/pipeon-vscode-extension/`** that you install into your fork or stock VS Code.

---

## Layers

| Layer | Role |
|--------|------|
| **Client** | VS Code–compatible shell (your **fork** of Code OSS, branded Pipeon). |
| **Pipeon extension** | In-IDE commands, panels, and future chat integration; talks to the worker / Ollama. |
| **Pipeon worker** | Aggregates artifacts, runs DockPipe/Docker when needed, routes inference. |
| **Ollama (host)** | Default Llama-class inference. |
| **Cloud (later)** | Optional accounts; same UX. |

---

## See also

- **`pipeon-ide-experience.md`** — UX and tone  
- **`pipeon-vscode-fork.md`** — how to fork VS Code and wire Pipeon  
