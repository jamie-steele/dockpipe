# Manual test plan — Windows

**Scope:** **`dockpipe.exe`** on the **Windows** host: **native** Docker + **bash** + git (default), optional **WSL bridge** (`DOCKPIPE_USE_WSL_BRIDGE=1`), **`windows setup` / `doctor`**, and **cold-start** behavior. Validate the Linux binary in WSL separately with **[manual-qa-core.md](manual-qa-core.md)** when testing the bridge.

**Who:** Maintainers and [platform-testing contributors](../CONTRIBUTING.md#platform-testing-we-need-you).

**Also see:** [install.md](../install.md) · [wsl-windows.md](../wsl-windows.md) · [cli-reference.md](../cli-reference.md) (Windows) · [manual-qa.md](manual-qa.md) (index)

---

## Prerequisites

**Native (default):**

- **Windows 10/11**, **Docker Desktop** (Windows `docker` on PATH), **Git for Windows** (or `git` + `bash` on PATH).
- **`dockpipe.exe`** on PATH — **MSI**, **[install.ps1](https://github.com/jamie-steele/dockpipe/blob/master/packaging/windows/install.ps1)**, release **zip**, or `GOOS=windows GOARCH=amd64 go build …`.

**WSL bridge (`DOCKPIPE_USE_WSL_BRIDGE=1`):**

- **WSL2** and at least one distro; **Docker** usable from that distro or via Desktop integration.
- **`dockpipe` installed inside WSL** (e.g. `dpkg -i` per [manual-qa-core.md](manual-qa-core.md) §1).

---

## 1. Native `dockpipe.exe` (no bridge)

- [ ] **PowerShell:** `cd` to a repo under **`C:\...`** (Docker Desktop running).
- [ ] Run: `dockpipe.exe -- echo ok` — **no** “Windows bridge” stderr line; command succeeds.
- [ ] Confirm **`DOCKPIPE_USE_WSL_BRIDGE`** is unset or not `1`.

---

## 2. `dockpipe.exe` + WSL (optional bridge)

Set **`$env:DOCKPIPE_USE_WSL_BRIDGE = "1"`** in PowerShell for this section.

Assumes WSL already has dockpipe and you have run **`dockpipe windows setup`** at least once (or you accept **first listed distro** fallback — see docs).

### 2.1 Bridge and cwd

- [ ] Put **`dockpipe.exe`** on `PATH` or use full path.
- [ ] **PowerShell:** `cd` to a repo under **`C:\...`**.
- [ ] Run: `dockpipe.exe -- echo ok`
- [ ] Stderr shows **Windows bridge** (distro + cwd mapping); inner command succeeds.

### 2.2 Path translation (flags)

- [ ] `--workdir` with a **Windows** path — inner argv should get **`/mnt/...`**, not raw `C:\`.
- [ ] `--mount` with Windows **host** path and Linux **container** path (e.g. `C:\data:/data`).
- [ ] Token **after** standalone **`--`** that looks like `C:\...` — should **not** be rewritten.

### 2.3 `windows` subcommand stays on Windows

- [ ] `dockpipe.exe windows doctor` does **not** forward into WSL like normal commands.
- [ ] `dockpipe.exe windows setup` (interactive or documented flags) — host-only.

---

## 3. Cold path (new user / new distro)

### 3.1 Reset Windows-side config (optional)

- [ ] Remove or rename **`%APPDATA%\dockpipe\windows-config.env`**.
- [ ] Optionally test fresh **`dockpipe.exe`** from release zip only.

### 3.2 New WSL distro (optional, strong signal)

- [ ] Add a distro (e.g. `wsl --install -d Ubuntu`).
- [ ] Inside it: **Docker** + **dockpipe** — follow **[manual-qa-core.md](manual-qa-core.md)** §1.

### 3.3 Setup + bridge from scratch

- [ ] `dockpipe.exe windows doctor` — distro list; behavior **without** config file (first distro fallback).
- [ ] `dockpipe.exe windows setup` — pin distro; **`--install-command`** if automating Linux install.
- [ ] With **`DOCKPIPE_USE_WSL_BRIDGE=1`**, from **`C:\...`**: `dockpipe.exe -- echo ok`.

---

## 4. Optional: workflow / bundle

- [ ] `dockpipe.exe --workflow …` from a Windows cwd (heavier; may need API keys).
- [ ] **Git bundle / fetch** flow (mostly relevant when mixing WSL + Windows git): [wsl-windows.md](../wsl-windows.md).

---

## 5. What to record if something fails

| Field | Example |
|--------|---------|
| Windows version | Win 11 23H2 |
| WSL distro | Ubuntu 22.04 |
| `uname -m` **inside WSL** | x86_64 / aarch64 |
| Linux install | `.deb` arch / tarball |
| `dockpipe.exe` origin | MSI / install.ps1 / zip / local build |
| Full command line | … |
| stderr + exit code | … |

---

## 6. Optional: MSI install smoke

- [ ] On a clean Windows profile (or VM), install **`dockpipe_<version>_windows_amd64.msi`** from the release (or build locally with **`packaging/msi/build.ps1`**).
- [ ] New shell: `dockpipe --help`
- [ ] Uninstall via **Apps & features** (or `msiexec /x {ProductCode}`) if you need a retest — PATH may retain an extra segment until edited manually (WiX util limitation).

---

## 7. Maintainer: artifacts to copy to Windows

**From CI / release:** use **`dockpipe_<version>_windows_amd64.msi`** + **`.deb`** + checksums.

**From Linux dev machine:**

```bash
go test ./...
./packaging/build-deb-all.sh <version>
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/dockpipe.exe ./cmd/dockpipe
```

On **Windows**, build MSI with **`packaging/msi/build.ps1`** (see **[packaging/msi/README.md](../../packaging/msi/README.md)**).

Copy **`packaging/build/dockpipe_<version>_amd64.deb`** (or arm64 if WSL is aarch64) and the **MSI or exe** to the Windows PC. Install the `.deb` **inside WSL** before testing the bridge.
