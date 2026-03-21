# Release notes template

Copy this file to **`releasenotes/X.Y.Z.md`** for the next release. Replace **`X.Y.Z`** everywhere (and **`vX.Y.Z`** in download URLs). Keep the **Linux → macOS → Windows** install order.

The GitHub Release workflow uses **`releasenotes/${VERSION}.md`** as the release body — include complete per-platform install steps below the “What’s new” section.

---

## Title line (example)

**X.Y.Z — Short summary of the release.**

---

## What's new

*(Changelog bullets, breaking changes, migration notes.)*

---

## Installation

Full reference: **[docs/install.md](https://github.com/jamie-steele/dockpipe/blob/vX.Y.Z/docs/install.md)**. Below: **Linux**, **macOS**, **Windows** for this tag (**vX.Y.Z**).

### Linux

**Prerequisites**

- **Docker** — required.
- **Bash** on the host — required (dockpipe always invokes bash).
- **git** on the host — for **clone / worktree / commit-on-host** only.

**Option A — `.deb` (recommended)**

**x86_64:**

```bash
wget https://github.com/jamie-steele/dockpipe/releases/download/vX.Y.Z/dockpipe_X.Y.Z_amd64.deb
sudo dpkg -i dockpipe_X.Y.Z_amd64.deb
```

**aarch64 (ARM64):**

```bash
wget https://github.com/jamie-steele/dockpipe/releases/download/vX.Y.Z/dockpipe_X.Y.Z_arm64.deb
sudo dpkg -i dockpipe_X.Y.Z_arm64.deb
```

If `dpkg` reports missing dependencies:

```bash
sudo apt-get install -f
```

**Option B — tarball**

```bash
# amd64
wget https://github.com/jamie-steele/dockpipe/releases/download/vX.Y.Z/dockpipe_X.Y.Z_linux_amd64.tar.gz
tar -xzf dockpipe_X.Y.Z_linux_amd64.tar.gz
sudo install -m 0755 dockpipe /usr/local/bin/dockpipe
```

```bash
# arm64
wget https://github.com/jamie-steele/dockpipe/releases/download/vX.Y.Z/dockpipe_X.Y.Z_linux_arm64.tar.gz
tar -xzf dockpipe_X.Y.Z_linux_arm64.tar.gz
sudo install -m 0755 dockpipe /usr/local/bin/dockpipe
```

**Option C — build from source**

Requires **Go** (see repo **`go.mod`**). From a clone at **`vX.Y.Z`**:

```bash
git clone https://github.com/jamie-steele/dockpipe.git
cd dockpipe && git checkout vX.Y.Z
make
export PATH="$PATH:$(pwd)/bin"
```

---

### macOS

**Prerequisites**

- **Docker Desktop for Mac** (or compatible engine) — required.
- **Bash** — required (`/bin/bash` is typical).
- **git** — for worktree / `--repo` / commit-on-host flows.

**Option A — Homebrew** (after the tap is published)

```bash
brew tap jamie-steele/dockpipe
brew install dockpipe
```

**Option B — release tarball**

**Apple Silicon (arm64):**

```bash
curl -LO https://github.com/jamie-steele/dockpipe/releases/download/vX.Y.Z/dockpipe_X.Y.Z_darwin_arm64.tar.gz
tar -xzf dockpipe_X.Y.Z_darwin_arm64.tar.gz
sudo install -m 0755 dockpipe /usr/local/bin/dockpipe
```

**Intel (amd64):**

```bash
curl -LO https://github.com/jamie-steele/dockpipe/releases/download/vX.Y.Z/dockpipe_X.Y.Z_darwin_amd64.tar.gz
tar -xzf dockpipe_X.Y.Z_darwin_amd64.tar.gz
sudo install -m 0755 dockpipe /usr/local/bin/dockpipe
```

**Option C — build from source**

```bash
git clone https://github.com/jamie-steele/dockpipe.git
cd dockpipe && git checkout vX.Y.Z
make
export PATH="$PATH:$(pwd)/bin"
```

---

### Windows

**Prerequisites**

- **Docker Desktop** — required.
- **`bash.exe`** on `PATH` — required. **Git for Windows** is the usual install (**`bash.exe` + `git.exe`**).
- **`git`** — additionally for worktrees, **`--repo`**, commit-on-host, etc.

**Option A — PowerShell install script**

```powershell
$i = "$env:TEMP\dockpipe-install.ps1"
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/jamie-steele/dockpipe/master/packaging/windows/install.ps1" -OutFile $i -UseBasicParsing
& $i -Version X.Y.Z
```

**Option B — MSI** — If **`dockpipe_X.Y.Z_windows_amd64.msi`** is attached to this release, install per-user (adds `%LOCALAPPDATA%\dockpipe` to **PATH**). If **no** `.msi` is published for this tag (e.g. MSI was skipped), write **“MSI — coming soon”** and point users to **Option A** + zip only.

**Option C — zip** — **`dockpipe_X.Y.Z_windows_amd64.zip`**: unzip, add to `PATH`, open a new terminal.

**Verify**

```powershell
dockpipe --version
dockpipe -- echo ok
```

**Optional — WSL bridge** (`DOCKPIPE_USE_WSL_BRIDGE=1`): install **`dockpipe`** in WSL, then **`dockpipe windows setup`**. See **[docs/wsl-windows.md](https://github.com/jamie-steele/dockpipe/blob/vX.Y.Z/docs/wsl-windows.md)**.

**Build from source on Windows**

```powershell
git clone https://github.com/jamie-steele/dockpipe.git
cd dockpipe
git checkout vX.Y.Z
$env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w -X main.Version=X.Y.Z" -o dockpipe.exe ./cmd/dockpipe
```

---

## Upgrade notes

*(From previous version — optional.)*

---

Feedback: [CONTRIBUTING.md](https://github.com/jamie-steele/dockpipe/blob/master/CONTRIBUTING.md)
