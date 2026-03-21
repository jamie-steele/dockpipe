# Installing dockpipe

**New to dockpipe?** Run **`dockpipe -- pwd`** after install, then read **[onboarding.md](onboarding.md)**.

**Platforms:** **Docker** and **bash** on the host are required everywhere. Linux is the primary target (`.deb`). macOS: Docker Desktop + bash (system `/bin/bash` is fine). Windows: **`dockpipe.exe`** + Docker Desktop + **Git for Windows** (bash + git). **`DOCKPIPE_USE_WSL_BRIDGE=1`** and **`dockpipe windows …`** are **optional** — only if you want the Linux `dockpipe` binary inside a WSL distro.

### Bundled templates (no extra install tree)

**`templates/`**, **`scripts/`**, **`images/`**, and **`lib/entrypoint.sh`** ship **inside** the `dockpipe` binary. On first use they unpack to the **user cache** (e.g. `~/.cache/dockpipe/bundled-<version>` on Linux, `%LocalAppData%\dockpipe\...` on Windows). You do **not** need a git clone of dockpipe next to the binary for **`--workflow`** or default images.

- **`DOCKPIPE_REPO_ROOT`** — optional override to point at a **dockpipe source tree** (e.g. when editing templates).
- **`DOCKPIPE_BUNDLED_CACHE`** — optional parent directory for the `dockpipe/bundled-*` folder (tests, custom cache location).

User-created files from **`dockpipe init`** / **`dockpipe template init`** still live on disk under the destination you choose.

---

## Install the .deb (Linux)

1. Download the latest `.deb` for your CPU from [Releases](https://github.com/jamie-steele/dockpipe/releases):
   - **x86_64** → `dockpipe_*_amd64.deb`
   - **aarch64** (ARM64 Linux, e.g. many cloud VMs / Raspberry Pi OS 64-bit) → `dockpipe_*_arm64.deb`  
   The two packages are **not** interchangeable (each contains a native Go binary). The `.deb` installs **`/usr/bin/dockpipe`** only (bundled assets are inside the binary; no `/usr/lib/dockpipe` layout).
2. Install:

   ```bash
   sudo dpkg -i dockpipe_*_amd64.deb    # or *_arm64.deb on aarch64
   ```

3. If `dpkg` reports missing dependencies (e.g. Docker):

   ```bash
   sudo apt-get install -f
   ```

Using `dpkg -i` avoids apt sandbox warnings when the .deb is in your home directory; `apt install ./file.deb` there can show a permission notice (apt’s `_apt` user can’t read the file).

**Upgrades:** download the new .deb (same arch as before) and run `sudo dpkg -i dockpipe_*_amd64.deb` or `dockpipe_*_arm64.deb` as appropriate.

**Requirements:** **amd64** or **arm64** package matching your machine. **Docker** (`docker.io` or `docker-ce`), **`bash`** on the host (required — dockpipe uses it), and **git** (for clone/worktree/commit-on-host only). Install Docker if needed:

```bash
sudo apt-get install docker.io
```

**Persistent data:** By default dockpipe mounts a named volume `dockpipe-data` at `/dockpipe-data` and sets `HOME` there so tool state (e.g. first-time login) persists. Use `--data-vol <name>`, `--data-dir /path`, or `--no-data` to change or disable. If a tool exits immediately with the default volume, try `--no-data` or `--reinit` to get a fresh volume.

**Workflow YAML:** Multi-step templates (`steps:`, async groups, `outputs:`) are documented in **[workflow-yaml.md](workflow-yaml.md)**.

---


## Or run from source (Linux or macOS, no root)

The CLI is built with **Go** matching **`go.mod`** (currently **1.25**; see `toolchain` there) (`go build -o bin/dockpipe.bin ./cmd/dockpipe` or **`make`**). The `bin/dockpipe` script runs the binary if present, otherwise `go run`.

```bash
git clone https://github.com/jamie-steele/dockpipe.git
cd dockpipe
make   # or: go build -o bin/dockpipe.bin ./cmd/dockpipe
export PATH="$PATH:$(pwd)/bin"
dockpipe -- ls -la
```

**Windows `dockpipe.exe` from a Unix dev machine:** from the **repo root**, run **`make build-windows`** — output is **`bin/dockpipe.exe`** (gitignored). Copy that file to your PC; do not rely on a hardcoded path on someone else’s machine.

---

## Windows (Docker Desktop + native `dockpipe.exe`)

**Required:** **Docker Desktop**, **`bash.exe`** on `PATH`, and **`dockpipe.exe`** on `PATH`. Dockpipe always invokes **bash** on the host; **Git for Windows** is the usual way to get **`bash.exe`** (and **`git.exe`**) together. Docker Desktop does **not** ship bash. If you use **WSL**, put **Git’s `…\Git\bin`** **before** `C:\Windows\System32` on `PATH` so **`bash`** is **Git Bash**, not **WSL’s** `bash.exe` (dockpipe prefers Git Bash when installed; otherwise it uses WSL path rules).

**Host `git`:** additionally required for **clone / worktree / commit-on-host** (e.g. **`--repo`**, **`clone-worktree.sh`**). Git for Windows covers that for most users.

**Local / gitignored config in worktrees** (e.g. `.env`, `appsettings.Development.json`): see **[worktree-include.md](worktree-include.md)** — use **`.dockpipe-worktreeinclude`** or **`.worktreeinclude`** so dockpipe copies those paths into the worktree after it is created.

You do **not** need a WSL distro or Linux `dockpipe` unless you opt into **`DOCKPIPE_USE_WSL_BRIDGE=1`** (or follow [wsl-windows.md](wsl-windows.md) for bundle/mixed-clone flows). If **`bash`** is missing but **WSL** is installed, the CLI may offer to **re-run through WSL** (interactive).

### Install `dockpipe.exe` on Windows

Add **`dockpipe.exe`** to `PATH` (**install script** or **zip**; **MSI** when published on a given release). Ensure **Docker Desktop** is running and **Git for Windows** (or another **`bash`** + **`git`** on `PATH`).

**Optional WSL bridge:** if you set **`DOCKPIPE_USE_WSL_BRIDGE=1`**, commands are forwarded into WSL. Then you also need **`dockpipe` installed inside that distro** and should run **`dockpipe windows setup`** once.

**Automated (recommended):** downloads from the latest release — prefers **MSI** when the release includes it, otherwise **zip** — verifies **`SHA256SUMS.txt`** when available, installs **per-user** (no admin):

```powershell
irm https://raw.githubusercontent.com/jamie-steele/dockpipe/master/packaging/windows/install.ps1 | iex
```

Pin a version: save [packaging/windows/install.ps1](https://github.com/jamie-steele/dockpipe/blob/master/packaging/windows/install.ps1) and run `.\install.ps1 -Version 0.6.0`.

**Manual:** from [Releases](https://github.com/jamie-steele/dockpipe/releases):

- **`dockpipe_<version>_windows_amd64.zip`** — unzip and add the folder to `PATH`.
- **`dockpipe_<version>_windows_amd64.msi`** — **when published** for that release: double-click, or `msiexec /i .\….msi /qn` (adds `%LOCALAPPDATA%\dockpipe` to your user **PATH**). Some releases ship **zip only** until MSI is enabled for that tag.

**winget:** not in the default Microsoft catalog until a manifest is accepted; see **[packaging/winget/README.md](../packaging/winget/README.md)** for maintainers and future `winget install`.

Open a **new** terminal after install so `PATH` is picked up.

### Daily use from Windows

With **`dockpipe.exe`** on `PATH` (install script, zip, or MSI when available). From **PowerShell or CMD**, `cd` to your repo and run the same CLI as on Linux, e.g.:

```powershell
cd C:\Users\you\src\myrepo
dockpipe -- echo ok
```

**Native mode (default):** `dockpipe` runs on Windows; **`docker`** comes from Docker Desktop; **`bash`** (and usually **`git`**) from Git for Windows. **`git`** is only needed for worktree/repo flows (see above). No WSL shell or Linux `dockpipe` required unless you use the bridge.

### Optional: WSL bridge (`DOCKPIPE_USE_WSL_BRIDGE=1`)

Set the environment variable **for the session** (or persist it in **Windows user environment variables** / your shell profile) so **`dockpipe.exe` forwards** into WSL: cwd is mapped with `wslpath`, then **`dockpipe`** runs inside the distro from **`dockpipe windows setup`** (or the first listed distro).

```powershell
# PowerShell — this session only
$env:DOCKPIPE_USE_WSL_BRIDGE = "1"
```

```bat
REM cmd.exe — this session only
set DOCKPIPE_USE_WSL_BRIDGE=1
```

- **`dockpipe windows …`** always runs **only on Windows** (setup / doctor).
- With the bridge, path-like flags are rewritten to WSL paths before the inner `dockpipe` sees them. Arguments after **`--`** are not rewritten.

**One-time WSL bootstrap** (only if you use the bridge):

```powershell
dockpipe windows setup
```

What setup does: picks a distro, saves it to `%APPDATA%\dockpipe\windows-config.env`, bootstraps `~/.dockpipe/windows-host.env` in WSL, optionally runs `--install-command`, verifies `dockpipe` in that distro.

```powershell
dockpipe windows setup --distro Ubuntu --install-command "<your install command>" --non-interactive
dockpipe windows doctor
```

**Manual QA:** **[manual-qa.md](qa/manual-qa.md)** — [Linux / `.deb` / WSL Linux](qa/manual-qa-core.md), [macOS](qa/manual-qa-macos.md), [Windows + `dockpipe.exe`](qa/manual-qa-windows.md).

---

## macOS (Homebrew)

**Requirements:** **Docker Desktop** (or another engine), and **`bash`** (dockpipe requires it; `/bin/bash` is normal). **git** for worktree / `--repo` flows.

Preferred path once tap is published:

```bash
brew tap jamie-steele/dockpipe
brew install dockpipe
```

Upgrade:

```bash
brew update
brew upgrade dockpipe
```

Maintainer note: formula source is tracked in `packaging/homebrew/dockpipe.rb` with release process in `packaging/homebrew/README.md`.

General release automation details: **[releases/releasing.md](releases/releasing.md)**.

---

## macOS (source fallback)

Current install path is source-based:

```bash
git clone https://github.com/jamie-steele/dockpipe.git
cd dockpipe
make
export PATH="$PATH:$(pwd)/bin"
```

To persist PATH on zsh (run from your **dockpipe** clone so `$(pwd)` is correct):

```bash
echo "export PATH=\"\$PATH:$(pwd)/bin\"" >> ~/.zshrc
```

## Building the .deb (for maintainers)

From the repo root:

```bash
./packaging/build-deb.sh [version] [amd64|arm64]   # default: 0.6.0 amd64
./packaging/build-deb-all.sh [version]             # both amd64 + arm64
# Output: packaging/build/dockpipe_<version>_{amd64,arm64}.deb
```

Attach that file to a GitHub Release. If we add a proper APT repo later, we’ll document it here.
