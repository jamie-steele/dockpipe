# `dockpipe-code-server` image

**Base:** [Coder](https://coder.com/)’s **`codercom/code-server`** (OSS VS Code in the browser).

**Adds:**

- **Pipeon** extension from **`src/contrib/pipeon-vscode-extension/`**, packaged with **`vsce`** in a **multi-stage** build (`.vsix`), then `code-server --install-extension`.
- **Browser tab icon**: **`favicon.ico` / `.svg`** (Pipeon **P** mark) copied over code-server’s default media paths inside the image.
- **Default User settings** (`user-settings.json`): telemetry off, no auto-update; **window title** prefix **Pipeon**; **`window.autoDetectColorScheme`** so light/dark follows the **browser/OS** `prefers-color-scheme` with **Default Dark+** / **Default Light+**. (Full **`product.json`** branding is for a **forked desktop** build, not this image.)

**Isolation:** Each **`docker run`** is a separate container; your project is only the **`/work`** bind-mount. Editor state and extensions live in the image (or container layer), not on the host — unlike a desktop VS Code profile.

**Not** the same as **`images/vscode/`** (`dockpipe-vscode`): that Dockerfile swaps in **`assets/entrypoint.sh`** for **`dockpipe --isolate vscode`** CLI runs. This **`code-server/`** image keeps the upstream **code-server** entrypoint for **`scripts/vscode/vscode-code-server.sh`** (canonical script under **`templates/core/resolvers/vscode/`**).

## Desktop shortcuts (cross-platform)

Prereqs: **`make build`** (or **`make build-windows`** + copy **`bin\dockpipe.exe`** to PATH on Windows), **`make build-code-server-image`**, and **`make pipeon-icons`** if favicons / PNG icon are missing.

| OS | Command | What you get |
|----|---------|----------------|
| **Linux** | `make install-pipeon-shortcut` | Freedesktop **`~/.local/share/applications/pipeon-code-server.desktop`** + **P** icon in **`~/.local/share/icons/hicolor/`** |
| **macOS** | `make install-pipeon-shortcut` or `make install-pipeon-shortcut-macos` | **`~/Applications/Pipeon.command`** (double-click opens Terminal; set custom Dock icon manually if you want) |
| **Windows** | `make install-pipeon-shortcut` from **Git Bash**, or **`make install-pipeon-shortcut-windows`**, or `powershell -NoProfile -ExecutionPolicy Bypass -File pipeon\scripts\install-pipeon-desktop-shortcut.ps1` | **`Pipeon.lnk`** on **Desktop** and under **Start Menu → Programs** with **`favicon.ico`** as the icon; target runs **`pipeon\scripts\pipeon-code-server-launch.ps1`** |

Workspace defaults to **user profile** (`$HOME` / **`%USERPROFILE%`**). Override: **`PIPEON_WORKDIR`** (bash/macOS) or **`PIPEON_WORKDIR`** env var before double-click (Windows: edit shortcut or set user environment variable).

**Windows notes:** Requires **Docker Desktop**, **Git for Windows** (**`bash.exe`**), and **`dockpipe.exe`** on **`PATH`** *or* **`bin\dockpipe.exe`** next to your clone. See **[docs/install.md](../../../../../docs/install.md)** (native **`dockpipe.exe`** vs optional **`DOCKPIPE_USE_WSL_BRIDGE`**).

## Build

From the repository root:

```bash
docker build -t dockpipe-code-server:latest -f templates/core/resolvers/code-server/assets/images/code-server/Dockerfile .
```

Or **`make build-code-server-image`**.

The **vscode** workflow defaults **`CODE_SERVER_IMAGE`** to **`dockpipe-code-server:latest`**; the host script skips **`docker pull`** for **`dockpipe-*`** images and errors with this build line if the image is missing.

To use **plain upstream** without Pipeon: set **`CODE_SERVER_IMAGE=codercom/code-server:latest`** in workflow vars or the environment.
