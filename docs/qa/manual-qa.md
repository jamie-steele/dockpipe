# Manual QA (before release)

CI runs **`go test ./...`**, **`dockpipe package test`**, and **`dockpipe workflow test`**; validate installs on real machines when packaging or Docker behavior changes.

**Install / concepts:** [install.md](../install.md) · [onboarding.md](../onboarding.md)

---

## Linux (`.deb` / tarball / WSL)

**Prereqs:** Docker, bash, arch-matched artifact (`amd64` vs `arm64`).

- [ ] **`.deb`:** `sudo dpkg -i dockpipe_<v>_<arch>.deb` → `which dockpipe` = `/usr/bin/dockpipe` → `dockpipe --help`, `dockpipe -- echo ok`
- [ ] **Upgrade:** install older `.deb` then newer; smoke test again
- [ ] **Tarball:** `tar -xzf …`, `chmod +x dockpipe`, `./dockpipe -- echo ok`
- [ ] **From source:** `make` or `go build -o src/bin/dockpipe.bin ./src/cmd`, `./src/bin/dockpipe -- echo ok`
- [ ] **Core:** `dockpipe --isolate base-dev -- echo ok` (or small bundled image)

---

## macOS

**Prereqs:** Docker Desktop (or Colima/Rancher), arch = **`darwin_arm64`** vs **`darwin_amd64`**.

- [ ] Extract release tarball, `chmod +x dockpipe`, `./dockpipe --help`, `./dockpipe -- echo ok`
- [ ] Optional: binary on `PATH` without `./`

---

## Windows

**Native:** `dockpipe.exe`, Docker Desktop, Git for Windows.

- [ ] `dockpipe.exe -- echo ok` (no WSL bridge unless testing bridge)
- [ ] **WSL bridge** (`DOCKPIPE_USE_WSL_BRIDGE=1`): PowerShell `cd` to `C:\…` repo → `dockpipe.exe -- echo ok` — stderr shows bridge; inner command OK

**Detail:** [wsl-windows.md](../wsl-windows.md) · [cli-reference.md](../cli-reference.md)

---

## If something fails

Record: OS version, `uname` / arch, install method (`deb` / tarball / source), `docker --version`, failing command + **stderr**.

**Packaging maintainer:** [release/packaging/homebrew/README.md](../../release/packaging/homebrew/README.md) · `release/packaging/build-deb-all.sh`

**Contributors:** [CONTRIBUTING.md](../../CONTRIBUTING.md#platform-testing-we-need-you)
