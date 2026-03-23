# Pipeon — forking VS Code (Code OSS)

The **VS Code** product is **not** vendored inside this repository (it is millions of lines and its own release cadence). Pipeon’s **editor shell** is implemented by maintaining a **separate fork** of **Code - OSS** (the open-source core of Visual Studio Code), then adding Pipeon branding, update endpoints, and the **Pipeon extension**.

This document is the **maintainer playbook**. It is not legal advice; respect **Microsoft’s trademarks** and the **vscode** license when you ship binaries.

---

## 1. Why a separate repo

| Approach | Notes |
|----------|--------|
| **Fork `microsoft/vscode` (or mirror)** | Full control; you build Electron, apply `product.json`, ship **Pipeon** as the app name in your build. |
| **Base on VSCodium** | Community builds without MS telemetry; some teams prefer it as a starting point—still a fork workflow. |
| **Extension-only (no fork)** | Ship **`src/contrib/pipeon-vscode-extension`** into **stock VS Code**—fastest path for dev; **not** a branded Pipeon desktop by itself. |

**Recommended path for “Pipeon IDE”:** **fork Code OSS** in a dedicated repo (e.g. `pipeon/pipeon-editor`), pin to a **vscode release tag**, apply patches, CI build per OS.

---

## 2. High-level steps (Code OSS)

1. **Clone** the upstream you chose (e.g. `git clone https://github.com/microsoft/vscode.git pipeon-editor && cd pipeon-editor`).
2. **Checkout** a stable **release tag** (not `main`), e.g. `git checkout 1.96.0` (use a current stable tag).
3. **Install** build deps per [upstream docs](https://github.com/microsoft/vscode/wiki/How-to-Contribute) (Node, Python, `yarn`, platform compilers).
4. **Customize `product.json`** (or the equivalent in your chosen fork) for:
   - **Application name** / **title** → Pipeon (follow trademark rules; many forks use a distinct name in the window title).
   - **Update URL / quality** → your own update channel when you have one.
   - **Extension gallery** → Open VSX or your own marketplace when ready.
5. **Build** (typical): `yarn` then `yarn gulp <your-platform>` — see upstream **README** for the exact target (`vscode-linux-x64`, `vscode-darwin-x64`, etc.).
6. **Install the Pipeon extension** into the built app (see §4).

**Resources:** [VS Code wiki — How to Contribute](https://github.com/microsoft/vscode/wiki/How-to-Contribute), [Code OSS branding notes](https://github.com/microsoft/vscode/wiki/Differences-between-the-repository-and-Visual-Studio-Code).

---

## 3. What to add in *your* fork (beyond branding)

| Addition | Purpose |
|----------|---------|
| **Bundled or recommended extension** | Ship **`src/contrib/pipeon-vscode-extension`** from **this** repo as a `.vsix` or built-in extension folder. |
| **Default settings / welcome** | Point users at Ollama, workspace folders, link to Pipeon docs. |
| **Update server** | When you ship Pipeon binaries, host your own update JSON or use a static download page first. |

**Worker:** The long-running **Pipeon worker** (HTTP or stdio next to the editor) usually lives in **another** service/repo or starts from the extension; it reuses the same artifact contracts as **`src/bin/pipeon`** / **`.dockpipe/pipeon-context.md`**.

---

## 4. Wire in the extension from *this* repo

```bash
cd /path/to/dockpipe/src/contrib/pipeon-vscode-extension
npm install
npx @vscode/vsce package   # produces pipeon-*.vsix
```

Install the `.vsix` into your Pipeon (or VS Code) build: **Extensions → … → Install from VSIX…**

For **development**, open **`src/contrib/pipeon-vscode-extension`** in VS Code and **F5** (Extension Development Host).

**Browser (Coder code-server):** this repo also ships **`dockpipe-code-server:latest`** — **Coder’s** `codercom/code-server` image with Pipeon pre-installed, baseline User settings, and a **Pipeon P-mark favicon** in the browser tab. Build: **`make build-code-server-image`** (see **`templates/core/resolvers/code-server/assets/images/code-server/README.md`**). The **vscode** workflow uses it by default (`dockpipe --workflow vscode`). Icon sources: **`make pipeon-icons`** → **`src/pipeon/scripts/generate-pipeon-icons.py`**.

---

## 5. Relationship to `dockpipe`

- **This repo** = DockPipe / DorkPipe **engine**, artifact **schemas**, **harness** (`src/bin/pipeon`), and **extension stub**.
- **Pipeon editor repo** = **fork** of Code OSS + CI + signing + your product updates.

Keep them **linked** in docs (version pins, extension version).

---

## 6. Trademarks and naming

- **“Visual Studio Code”** and related marks are **Microsoft** trademarks.
- **Code - OSS** builds must not misrepresent origin; use **your** product name (**Pipeon**) for the shipped app where appropriate and follow upstream license terms.

When in doubt, consult the **LICENSE** in the vscode repository and your counsel.
