# WSL profile for Dockpipe (optional bridge)

Dockpipe’s **default Windows path** is **native** `dockpipe.exe` + **Docker Desktop** + **Git for Windows** — no Linux distro required.

If you use **`DOCKPIPE_USE_WSL_BRIDGE=1`**, you need a Linux **`dockpipe`** binary inside WSL. This repo standardizes on a **minimal Alpine** side distro when possible.

## Why Alpine

- **Small** root filesystem and **`apk`** package set.
- **musl** is fine: release **Linux** binaries are **static** Go builds (`CGO_ENABLED=0`) and run on Alpine.
- **Docker**: install **`docker-cli`** in Alpine and point **`DOCKER_HOST`** at Docker Desktop (or mount the socket) — same pattern as other slim WSL setups.

## Automated install

From Windows (after `dockpipe.exe` is on `PATH`):

```powershell
dockpipe windows setup --bootstrap-wsl --distro Alpine --non-interactive --install-dockpipe
```

Or use **`release/packaging/windows/install.ps1`**, which runs that after installing the Windows binary (unless **`-SkipWSLSetup`**).

The embedded script installs a minimal set of packages on Alpine:

`bash`, `curl`, `ca-certificates`, `git`, `tar`, `gzip`, `docker-cli`

Then it downloads the latest **`dockpipe_*_linux_*.tar.gz`** from GitHub into **`~/.local/bin`**.

## If Alpine is not offered

Some Windows builds do not list **Alpine** in `wsl --list --online`. Use **Ubuntu** instead:

```powershell
dockpipe windows setup --bootstrap-wsl --distro Ubuntu --non-interactive --install-dockpipe
```

## Docker from Alpine WSL

Ensure **Docker Desktop** is running with **WSL 2 integration** enabled for your distro. The **`docker-cli`** package talks to the engine via the usual socket / `DOCKER_HOST`; see Docker Desktop docs if commands fail from inside WSL.

## Forks / custom releases

Set **`DOCKPIPE_GITHUB_REPO=owner/repo`** in the WSL environment before running the install step, or pass a custom **`--install-command`** to **`dockpipe windows setup`**.
