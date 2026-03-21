# Manual test plan — core (Linux `.deb` / tarballs)

**Scope:** Validates **Linux packaging** and **portable behavior** of the Go CLI on **native Linux** or **inside WSL** (same `.deb` and same checks). Does **not** cover the Windows `dockpipe.exe` host — see **[manual-qa-windows.md](manual-qa-windows.md)** (native + optional WSL bridge); use this doc when the bridge forwards into WSL and you need Linux `dockpipe` inside the distro.

**Who:** Maintainers and [platform-testing contributors](../CONTRIBUTING.md#platform-testing-we-need-you).

**Also see:** [install.md](../install.md) · [manual-qa.md](manual-qa.md) (index)

---

## Prerequisites

- **Docker** available (`docker run hello-world` or equivalent).
- **Bash** and **git** (for flows that commit on host).
- Artifacts matching **CPU arch** (`uname -m`):
  - `x86_64` → **amd64** `.deb` or `linux_amd64` tarball
  - `aarch64` → **arm64** `.deb` or `linux_arm64` tarball

---

## 1. Debian package (`.deb`)

Validates layout under **`/usr/lib/dockpipe`**, symlink **`/usr/bin/dockpipe`**, and **upgrade** behavior.

### 1.1 Fresh install

- [ ] Copy `dockpipe_<version>_amd64.deb` or `*_arm64.deb` onto the machine (from `/mnt/c/...` in WSL is fine).
- [ ] `sudo dpkg -i ./dockpipe_<version>_<arch>.deb`
- [ ] If needed: `sudo apt-get install -f`
- [ ] `which dockpipe` → `/usr/bin/dockpipe`
- [ ] `dockpipe --help` (no crash, usage visible).
- [ ] `dockpipe -- echo ok` succeeds.

### 1.2 Upgrade over an existing install

- [ ] Install an **older** `.deb` first (optional).
- [ ] `sudo dpkg -i` the **new** `.deb` (**same** Debian arch as before).
- [ ] `dockpipe --help` and `dockpipe -- echo ok` still work.

### 1.3 Wrong arch must fail predictably

- [ ] On **amd64** machine, do **not** expect `*_arm64.deb` to run; on **arm64**, do **not** install `*_amd64.deb` as a “test” on production — use matching arch only.

---

## 2. Linux tarball (optional)

Release artifact: `dockpipe_<version>_linux_amd64.tar.gz` or `_linux_arm64.tar.gz`.

- [ ] Extract: `tar -xzf dockpipe_<version>_linux_<arch>.tar.gz`
- [ ] `chmod +x dockpipe` if needed.
- [ ] `./dockpipe --help` and `./dockpipe -- echo ok` from the extract directory (or add to `PATH`).

---

## 3. From source (developer smoke)

- [ ] `go test ./...`
- [ ] `make` or `go build -o bin/dockpipe.bin ./cmd/dockpipe`
- [ ] `PATH=$PWD/bin:$PATH dockpipe -- echo ok`

---

## 4. Core CLI (any install method)

- [ ] **Attached run:** `dockpipe -- echo ok`
- [ ] **Detach (quick):** `dockpipe -d -- sleep 5` then confirm container stops (optional).
- [ ] **Isolate:** `dockpipe --isolate base-dev -- echo ok` (or another small image you ship).

---

## 5. Optional: workflow / template (heavier)

- [ ] `dockpipe --workflow <bundled-name> -- …` minimal command (if you have credentials / network for tools).
- [ ] Or `dockpipe init` / `dockpipe template init` into a temp dir (no network if using `--from` bundled name only).

---

## 6. What to record if something fails

| Field | Example |
|--------|---------|
| OS | Ubuntu 22.04 / WSL Ubuntu |
| `uname -m` | x86_64 / aarch64 |
| Install | `.deb` amd64 / tarball / source |
| `docker --version` | … |
| Command + stderr | … |

---

## 7. Maintainer: local build artifacts

```bash
go test ./...
./packaging/build-deb-all.sh <version>    # packaging/build/dockpipe_<version>_{amd64,arm64}.deb
# Tarballs: match .github/workflows/release.yml (linux_amd64 / linux_arm64 go build + tar)
```

**Next (Windows host):** after WSL has dockpipe installed via §1, run **[manual-qa-windows.md](manual-qa-windows.md)**.
