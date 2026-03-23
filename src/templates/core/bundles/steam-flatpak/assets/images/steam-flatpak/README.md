# `steam-flatpak` image

Debian **bookworm-slim** with **Flatpak** and **Flathub `com.valvesoftware.Steam`** (system install). Uses the shared **`lib/entrypoint.sh`**.

## Why a host script?

Flatpak uses **bubblewrap** and expects a usable session (D-Bus, GPU, display). Inside Docker that usually means **`--privileged`** (or a very long list of caps/seccomp tweaks) plus mounts for **X11 or Wayland**, **`/dev/dri`**, and often the **user D-Bus** socket. Do **not** run Steam as root on the host; in-container UID mapping is handled by `docker run -u`.

**Do not** expect this to work on macOS/Windows Docker Desktop without extra plumbing; it is aimed at **Linux desktops**.

## Build (repo root)

```bash
docker build -t dockpipe-steam-flatpak:$(tr -d '\n' < VERSION) \
  -f templates/core/bundles/steam-flatpak/assets/images/steam-flatpak/Dockerfile .
```

First build downloads the Steam Flatpak payload (large).

## Run (prefer `scripts/docker-steam-flatpak.sh`)

```bash
./scripts/docker-steam-flatpak.sh
```

Optional: `DOCKER_STEAM_NVIDIA=1` adds `--gpus all`. Override image with `DOCKER_STEAM_IMAGE=dockpipe-steam-flatpak:tag`.

**Caches:** by default the script mounts a named volume **`dockpipe-flatpak-system` → `/var/lib/flatpak`** so Flathub runtimes survive relaunches. Disable with `DOCKER_STEAM_USE_FLATPAK_CACHE_VOLUME=0` or change the name with `DOCKER_STEAM_FLATPAK_CACHE_VOLUME`.

## dockpipe

- **`TemplateBuild`**: `--isolate steam-flatpak` builds this image (same as other templates).
- **Workflow**: `steam-flatpak-docker` runs the host script (`skip_container: true`) so mounts stay in one place.

Licensing: Steam is **Valve** proprietary; Flatpak pulls **official** Flathub/Valve paths when you install. This image does not redistribute Steam outside that flow.
