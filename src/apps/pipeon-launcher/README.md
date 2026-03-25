# Pipeon host launcher

Cross-platform **Qt 6** system-tray app: save **contexts** (folder + resolver / strategy / runtime), **launch** or **stop** `dockpipe` subprocesses, **open logs** and folders. It does **not** run workflows inside the GUI; all execution stays in **DockPipe** (and optionally **DorkPipe** later).

## Requirements

- **CMake** 3.16+
- **Qt 6** (`Widgets` + **`Network`**) — install dev packages (e.g. `qt6-base-dev` on Debian/Ubuntu) or use the [Qt Online Installer](https://www.qt.io/download) and set **`CMAKE_PREFIX_PATH`** to the Qt 6 prefix.
- **OpenGL / EGL development libraries** — Qt 6 Gui pulls in **WrapOpenGL**. On Ubuntu/Pop!_OS, if CMake says `WrapOpenGL could not be found` or `Qt6Gui_FOUND` is FALSE, install **`libgl1-mesa-dev`** and **`libegl1-mesa-dev`** (see Build section below).
- **`dockpipe`** on **`PATH`** (or set **dockpipe binary** in each context’s settings).
- Host tools DockPipe already needs: **`bash`**, **`docker`**, **`git`** — see [docs/install.md](../../docs/install.md).

## Build

`CMakeLists.txt` lives under **`src/apps/pipeon-launcher/`**, not the dockpipe repo root. Run CMake with that directory as the **source** (or `cd` there first).

**Fastest — from the repo root:** `make pipeon-launcher` (writes **`src/apps/pipeon-launcher/build/`**).

### Pop!_OS / Linux: Applications menu shortcut

`make install-pipeon-shortcut` installs a **different** entry (code-server in the browser). For this **Qt tray app**, build first, then install the Freedesktop file:

```bash
make pipeon-launcher
make install-pipeon-launcher-shortcut
```

That creates **`~/.local/share/applications/pipeon-launcher.desktop`**. Search the app menu for **Pipeon Launcher** (you may need to log out and back in). To install **both** shortcuts on Linux: **`make install-pipeon-all-shortcuts`**.

**Option A — from the repo root (CMake by hand):**

```bash
cd ~/source/dockpipe
sudo apt install cmake build-essential qt6-base-dev   # Pop!_OS / Ubuntu: Qt 6 Widgets + dev tools
cmake -S src/apps/pipeon-launcher -B src/apps/pipeon-launcher/build
cmake --build src/apps/pipeon-launcher/build
./src/apps/pipeon-launcher/build/pipeon-launcher
```

**Option B — from the launcher directory:**

```bash
cd ~/source/dockpipe/src/apps/pipeon-launcher
cmake -B build
cmake --build build
./build/pipeon-launcher
```

If you use the **Qt Online Installer** instead of distro packages, point CMake at that kit (replace with your real path):

```bash
cmake -S src/apps/pipeon-launcher -B src/apps/pipeon-launcher/build \
  -DCMAKE_PREFIX_PATH="$HOME/Qt/6.8.0/gcc_64"
```

Do **not** use the placeholder `/path/to/Qt/6.x/...` literally — it must be a directory that contains **`Qt6Config.cmake`** (or install `qt6-base-dev` and omit `CMAKE_PREFIX_PATH`).

**If configuration failed earlier** (stale cache): remove the build dir and re-run CMake after installing `libgl1-mesa-dev` / `libegl1-mesa-dev`:

```bash
rm -rf src/apps/pipeon-launcher/build
cmake -S src/apps/pipeon-launcher -B src/apps/pipeon-launcher/build
cmake --build src/apps/pipeon-launcher/build
```

## LGPL / Qt

Qt is available under **LGPL** and commercially. If you **ship binaries**, comply with Qt’s license terms (e.g. dynamic linking and relinking for LGPL) or use a **commercial Qt license**. This README is not legal advice.

## Flathub search & extra `dockpipe` env

**Edit context…** includes **Extra dockpipe env** (one `KEY=value` per line). Each line is passed to dockpipe as **`--env`**, same as the CLI — no dockpipe core changes.

- **Browse Flathub…** opens a search dialog (POST to **`https://flathub.org/api/v2/search`**) with optional **category** filter (client-side on `main_categories`). Choosing an app sets **`FLATHUB_APP_ID=`** plus the Flatpak app id (replaces an existing line with that key).

Use the **`flathub-host`** workflow (on disk **`.staging/workflows/flathub-host/`** in this repo) with **`scripts/flathub-host/flathub-host-run.sh`**, which runs **`flatpak run`** on the **host** (Linux with Flatpak + Flathub installed). **Docker**-based Flathub flows and **named volume caches** stay in separate workflows/scripts (`steam-flatpak-docker`, `package-cache-demo`).

**APT** (or other package managers) does not need a separate browser: add env lines your scripts read, or pass **`dockpipe --mount`** for cache volumes when you run a container-backed workflow from the CLI; Pipeon can mirror those with extra env lines only if your wrapper reads them.

## Basic vs Advanced

- **Basic** (default): **File → Open project folder…** (or **Choose folder…**) sets the project directory passed to `dockpipe` as **`--workdir`** (your code is mounted in the tool’s container). The main area lists only workflows whose workflow YAML includes **`category: app`** (see `docs/workflow-yaml.md`) — GUI/IDE-style apps. Double-click an app to launch. **Set up Cursor MCP** runs **`cursor-prep.sh` only** (writes **`.dockpipe/cursor-dev/`** hints; **no** Docker, **no** full `dockpipe` session). For a **Docker session container + Cursor on the host**, double-click the **`cursor-dev`** app — not the MCP button. **Refresh apps** (toolbar) or **File → Refresh app list** (**F5**) rescans `workflows/` and `src/templates/` or `templates/` from disk so new or edited workflows appear without restarting. **View → Icon grid** / **Compact list** toggles presentation. Mode and view are stored in **`launcher.json`**.
- **Advanced**: **View → Advanced mode** shows the full **context** list (same as before): **Add folder…** can import every workflow under the resolved repo; technical details per row; **Edit**, worktrees, logs, etc.

## Add folder (Advanced)

Choosing **Add folder…** resolves a dockpipe **repo root** from the path (`DOCKPIPE_REPO_ROOT` or walking upward for `workflows` / `.staging/workflows` / `src/templates/core` or `templates/core`). For each workflow with a `config.yml` under `workflows/...`, `.staging/workflows/...`, and `src/templates/...` or `templates/...` (excluding `core`), the launcher adds **one context** with that **workdir** and the matching `--workflow` name. If no repo is found, it adds a single context with workflow `vscode`. Existing `(workdir, workflow, workflow file)` combinations are skipped.

## Data locations

| OS      | Config / contexts                          |
|---------|---------------------------------------------|
| Linux   | `~/.config/pipeon/` (XDG)                  |
| macOS   | `~/Library/Application Support/Pipeon/`  |
| Windows | `%APPDATA%\Pipeon\`                      |

- **`contexts.json`** — saved contexts.
- **`launcher.json`** — UI mode (`basic` / `advanced`), Basic view (`icons` / `list`), last **project folder** for Basic mode.
- **`logs/`** — per-launch log files for `dockpipe` stdout/stderr.

## Manual QA (short)

1. **Tray:** Icon appears; Show / hide window; Quit exits the app.
2. **Add folder:** New context; **Launch** starts `dockpipe` (requires image/build for workflows like `vscode`).
3. **Parallel:** Two contexts, different workdirs — both **Launch**; both show **running**; **Stop** each.
4. **Git:** **Refresh worktrees** on a repo with multiple worktrees adds missing paths as contexts.
5. **Stop all for repo:** Stops every **running** context whose `git rev-parse --show-toplevel` matches (includes linked worktrees that live outside the main checkout directory).
6. **Linux:** Verify tray under **X11** and **Wayland** (may depend on desktop).

## UI

The window uses **Qt Fusion** plus stylesheets embedded in `pipeon.qrc`: shared `resources/theme/pipeon.qss` plus **`pipeon-light.qss`** or **`pipeon-dark.qss`**. Light/dark is chosen from **`QStyleHints::colorScheme`** when available (Qt 6.5+), and on **Linux** also from **`gsettings`** (`org.gnome.desktop.interface` color-scheme / gtk-theme), **`~/.config/gtk-3.0/settings.ini`** (`gtk-application-prefer-dark-theme`), **KDE** `~/.config/kdeglobals` `ColorScheme`, **`GTK_THEME`**, then palette luminance as a last resort. When dark is selected, the app applies a **Fusion dark palette** so backgrounds match the stylesheet (Qt often defaults to a light palette on Linux). **Light** mode uses stronger text/badge contrast. The stylesheet is **re-applied when the system color scheme changes** (Qt 6.5+). The main header groups **primary** session actions (launch, relaunch, stop, add folder) separately from **secondary** utilities (edit, refresh worktrees, logs, folder, remove, stop all for repo). The context list uses row widgets with a status badge; **Edit context** opens a grouped dialog with combo boxes populated from the dockpipe repo when `DOCKPIPE_REPO_ROOT` or the context workdir resolves to a checkout.

## Scope

Per design: the launcher only **controls** sessions. It does **not** replace **DorkPipe** orchestration or embed Ollama/containers.
