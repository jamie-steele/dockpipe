# Installing dockpipe

**Platforms:** Linux is the primary target (`.deb` package). macOS is supported from source (Bash + Docker). Windows uses **`dockpipe.exe` natively** (Docker Desktop + Git for Windows); optional **WSL forwarding** via `DOCKPIPE_USE_WSL_BRIDGE=1` if you prefer Linux git inside a distro.

---

## Install the .deb (Linux)

1. Download the latest `.deb` for your CPU from [Releases](https://github.com/jamie-steele/dockpipe/releases):
   - **x86_64** → `dockpipe_*_amd64.deb`
   - **aarch64** (ARM64 Linux, e.g. many cloud VMs / Raspberry Pi OS 64-bit) → `dockpipe_*_arm64.deb`  
   The two packages are **not** interchangeable (each contains a native Go binary).
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

**Requirements:** **amd64** or **arm64** package matching your machine. Bash, Docker (`docker.io` or `docker-ce`), and **git** (for commit-on-host). Install Docker if needed:

```bash
sudo apt-get install docker.io
```

**Persistent data:** By default dockpipe mounts a named volume `dockpipe-data` at `/dockpipe-data` and sets `HOME` there so tool state (e.g. first-time login) persists. Use `--data-vol <name>`, `--data-dir /path`, or `--no-data` to change or disable. If a tool exits immediately with the default volume, try `--no-data` or `--reinit` to get a fresh volume.

**Workflow YAML:** Multi-step templates (`steps:`, async groups, `outputs:`) are documented in **[workflow-yaml.md](workflow-yaml.md)**.

---


## Or run from source (Linux or macOS, no root)

The CLI is built with **Go 1.22+** (`go build -o bin/dockpipe.bin ./cmd/dockpipe` or **`make`**). The `bin/dockpipe` script runs the binary if present, otherwise `go run`.

```bash
git clone https://github.com/jamie-steele/dockpipe.git
cd dockpipe
make   # or: go build -o bin/dockpipe.bin ./cmd/dockpipe
export PATH="$PATH:$(pwd)/bin"
dockpipe -- ls -la
```

---

## Windows (Docker Desktop + native `dockpipe.exe`)

**What you actually need:** **Docker Desktop** is enough for *containers*, but dockpipe also runs **`git`** and **`bash` on the Windows host** (worktrees, commit-on-host, sourcing pre-scripts). Docker Desktop does **not** put `git` / `bash` on your PATH. In practice that means **two installs**: **Docker Desktop** + **Git for Windows** (covers git + bash). You do **not** need a separate WSL distro or Linux `dockpipe` unless you opt into **`DOCKPIPE_USE_WSL_BRIDGE=1`**.

### Install `dockpipe.exe` on Windows

Add the **Windows** binary to `PATH` (MSI, install script, or zip). Ensure **Docker Desktop** is running and **`git`** / **`bash`** are on PATH (Git for Windows is the usual choice).

**Optional WSL bridge:** if you set **`DOCKPIPE_USE_WSL_BRIDGE=1`**, commands are forwarded into WSL. Then you also need **`dockpipe` installed inside that distro** and should run **`dockpipe windows setup`** once.

**Automated (recommended):** downloads the latest release **MSI** (or zip fallback), verifies **`SHA256SUMS.txt`** when available, installs **per-user** (no admin):

```powershell
irm https://raw.githubusercontent.com/jamie-steele/dockpipe/master/packaging/windows/install.ps1 | iex
```

Pin a version: save [packaging/windows/install.ps1](https://github.com/jamie-steele/dockpipe/blob/master/packaging/windows/install.ps1) and run `.\install.ps1 -Version 0.6.0`.

**Manual:** from [Releases](https://github.com/jamie-steele/dockpipe/releases):

- **`dockpipe_<version>_windows_amd64.msi`** — double-click, or `msiexec /i .\….msi /qn` (adds `%LOCALAPPDATA%\dockpipe` to your user **PATH**).
- **`dockpipe_<version>_windows_amd64.zip`** — unzip and add the folder to `PATH`.

**winget:** not in the default Microsoft catalog until a manifest is accepted; see **[packaging/winget/README.md](../packaging/winget/README.md)** for maintainers and future `winget install`.

Open a **new** terminal after install so `PATH` is picked up.

### Daily use from Windows

With **`dockpipe.exe`** on `PATH` (MSI, install script, or zip). From **PowerShell or CMD**, `cd` to your repo and run the same CLI as on Linux, e.g.:

```powershell
cd C:\Users\you\src\myrepo
dockpipe -- echo ok
```

**Native mode (default):** `dockpipe` runs on Windows; **`docker`** comes from Docker Desktop, **`git`** / **`bash`** from Git for Windows (or your own tooling). No WSL shell or Linux `dockpipe` required.

### Optional: WSL bridge (`DOCKPIPE_USE_WSL_BRIDGE=1`)

Set the environment variable **for the session** (or in your profile) so **`dockpipe.exe` forwards** into WSL: cwd is mapped with `wslpath`, then **`dockpipe`** runs inside the distro from **`dockpipe windows setup`** (or the first listed distro).

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

**Manual QA:** **[manual-qa.md](manual-qa.md)** — [Linux / `.deb` / WSL Linux](manual-qa-core.md), [macOS](manual-qa-macos.md), [Windows + `dockpipe.exe`](manual-qa-windows.md).

---

## macOS (Homebrew)

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

General release automation details: `docs/releasing.md`.

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
