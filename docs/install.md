# Installing dockpipe

**Platforms:** Linux is the primary target (`.deb` package). macOS is supported from source (Bash + Docker). Windows is supported via **WSL2** with `dockpipe windows setup`.

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

## Windows (WSL2)

### Install `dockpipe.exe` on Windows

You need the **Windows** binary on `PATH` before `dockpipe windows setup` and before using the WSL bridge.

**Automated (recommended):** downloads the latest release **MSI** (or zip fallback), verifies **`SHA256SUMS.txt`** when available, installs **per-user** (no admin):

```powershell
irm https://raw.githubusercontent.com/jamie-steele/dockpipe/main/packaging/windows/install.ps1 | iex
```

Pin a version: save [packaging/windows/install.ps1](https://github.com/jamie-steele/dockpipe/blob/main/packaging/windows/install.ps1) and run `.\install.ps1 -Version 0.6.0`.

**Manual:** from [Releases](https://github.com/jamie-steele/dockpipe/releases):

- **`dockpipe_<version>_windows_amd64.msi`** — double-click, or `msiexec /i .\….msi /qn` (adds `%LOCALAPPDATA%\dockpipe` to your user **PATH**).
- **`dockpipe_<version>_windows_amd64.zip`** — unzip and add the folder to `PATH`.

**winget:** not in the default Microsoft catalog until a manifest is accepted; see **[packaging/winget/README.md](../packaging/winget/README.md)** for maintainers and future `winget install`.

Open a **new** terminal after install so `PATH` is picked up.

### One-time WSL bootstrap (`windows setup`)

From PowerShell or CMD on Windows:

```powershell
dockpipe windows setup
```

What setup does:

1. Detects available WSL distros (`wsl.exe -l -q`).
2. Prompts for distro selection (or uses `--distro <name>`).
3. Saves chosen distro in `%APPDATA%\dockpipe\windows-config.env`.
4. Bootstraps `~/.dockpipe/windows-host.env` in WSL and sources it from `.bashrc`.
5. Optionally runs your install command in WSL via `--install-command "<cmd>"`.
6. Verifies `dockpipe` exists in the selected distro.

Automation-friendly setup:

```powershell
dockpipe windows setup --distro Ubuntu --install-command "<your install command>" --non-interactive
```

Diagnostics:

```powershell
dockpipe windows doctor
```

### Daily use from Windows (no WSL shell required)

With **`dockpipe.exe`** on `PATH` (MSI, install script, or zip). From **PowerShell or CMD**, run the same commands you would in Linux, e.g.:

```powershell
cd C:\Users\you\src\myrepo
dockpipe -- echo ok
```

The Windows binary **forwards** argv into WSL: it maps your **current Windows directory** to a WSL path (`wslpath`), `cd`s there, and runs **`dockpipe`** inside the distro from `windows setup` (or the first listed distro if you have not run setup yet).

- **`dockpipe windows …`** still runs **only on Windows** (setup / doctor).
- **Paths in flags** (`--workdir`, `--mount`, `--env` / `--var` when the value is a path, etc.): you can use **`C:\…` / `D:\…`**, **UNC** (`\\server\share\…`), or **`/mnt/c/…`**; the bridge rewrites obvious Windows filesystem paths before calling dockpipe in WSL (`wslpath` when it works, otherwise a safe fallback). Arguments after **`--`** (the inner command) are not rewritten.

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

To persist PATH on zsh:

```bash
echo 'export PATH="$PATH:$HOME/source/dockpipe/bin"' >> ~/.zshrc
```

## Building the .deb (for maintainers)

From the repo root:

```bash
./packaging/build-deb.sh [version] [amd64|arm64]   # default: 0.6.0 amd64
./packaging/build-deb-all.sh [version]             # both amd64 + arm64
# Output: packaging/build/dockpipe_<version>_{amd64,arm64}.deb
```

Attach that file to a GitHub Release. If we add a proper APT repo later, we’ll document it here.
