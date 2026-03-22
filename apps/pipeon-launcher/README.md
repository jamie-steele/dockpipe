# Pipeon host launcher

Cross-platform **Qt 6** system-tray app: save **contexts** (folder + resolver / strategy / runtime), **launch** or **stop** `dockpipe` subprocesses, **open logs** and folders. It does **not** run workflows inside the GUI; all execution stays in **DockPipe** (and optionally **DorkPipe** later).

## Requirements

- **CMake** 3.16+
- **Qt 6** (`Widgets` module) — install dev packages (e.g. `qt6-base-dev` on Debian/Ubuntu) or use the [Qt Online Installer](https://www.qt.io/download) and set **`CMAKE_PREFIX_PATH`** to the Qt 6 prefix.
- **OpenGL / EGL development libraries** — Qt 6 Gui pulls in **WrapOpenGL**. On Ubuntu/Pop!_OS, if CMake says `WrapOpenGL could not be found` or `Qt6Gui_FOUND` is FALSE, install **`libgl1-mesa-dev`** and **`libegl1-mesa-dev`** (see Build section below).
- **`dockpipe`** on **`PATH`** (or set **dockpipe binary** in each context’s settings).
- Host tools DockPipe already needs: **`bash`**, **`docker`**, **`git`** — see [docs/install.md](../../docs/install.md).

## Build

`CMakeLists.txt` lives under **`apps/pipeon-launcher/`**, not the dockpipe repo root. Run CMake with that directory as the **source** (or `cd` there first).

**Option A — from the repo root:**

```bash
cd ~/source/dockpipe
sudo apt install cmake build-essential qt6-base-dev   # Pop!_OS / Ubuntu: Qt 6 Widgets + dev tools
cmake -S apps/pipeon-launcher -B apps/pipeon-launcher/build
cmake --build apps/pipeon-launcher/build
./apps/pipeon-launcher/build/pipeon-launcher
```

**Option B — from the launcher directory:**

```bash
cd ~/source/dockpipe/apps/pipeon-launcher
cmake -B build
cmake --build build
./build/pipeon-launcher
```

If you use the **Qt Online Installer** instead of distro packages, point CMake at that kit (replace with your real path):

```bash
cmake -S apps/pipeon-launcher -B apps/pipeon-launcher/build \
  -DCMAKE_PREFIX_PATH="$HOME/Qt/6.8.0/gcc_64"
```

Do **not** use the placeholder `/path/to/Qt/6.x/...` literally — it must be a directory that contains **`Qt6Config.cmake`** (or install `qt6-base-dev` and omit `CMAKE_PREFIX_PATH`).

**If configuration failed earlier** (stale cache): remove the build dir and re-run CMake after installing `libgl1-mesa-dev` / `libegl1-mesa-dev`:

```bash
rm -rf apps/pipeon-launcher/build
cmake -S apps/pipeon-launcher -B apps/pipeon-launcher/build
cmake --build apps/pipeon-launcher/build
```

## LGPL / Qt

Qt is available under **LGPL** and commercially. If you **ship binaries**, comply with Qt’s license terms (e.g. dynamic linking and relinking for LGPL) or use a **commercial Qt license**. This README is not legal advice.

## Add folder

Choosing **Add folder…** resolves a dockpipe **repo root** from the path (`DOCKPIPE_REPO_ROOT` or walking upward for `dockpipe/workflows` / `templates/core`). For each workflow with a `config.yml` under `dockpipe/workflows/...` and `templates/...` (excluding `templates/core`), the launcher adds **one context** with that **workdir** and the matching `--workflow` name. If no repo is found, it adds a single context with workflow `vscode`. Existing `(workdir, workflow, workflow file)` combinations are skipped.

## Data locations

| OS      | Config / contexts                          |
|---------|---------------------------------------------|
| Linux   | `~/.config/pipeon/` (XDG)                  |
| macOS   | `~/Library/Application Support/Pipeon/`  |
| Windows | `%APPDATA%\Pipeon\`                      |

- **`contexts.json`** — saved contexts.
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
