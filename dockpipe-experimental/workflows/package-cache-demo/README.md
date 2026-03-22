# `package-cache-demo` workflow

**Purpose:** Show how to **reuse Docker named volumes** so **Flathub** (`/var/lib/flatpak`) and **APT** (`/var/cache/apt`, `/var/lib/apt/lists`) data survive **container relaunches** and **image rebuilds** — without adding orchestration to dockpipe **core**.

## Run

```bash
dockpipe --workflow package-cache-demo
```

Optional: actually run `apt-get update` inside `debian:bookworm-slim` using the cached volumes:

```bash
DOCKER_PACKAGE_CACHE_DEMO_RUN=1 dockpipe --workflow package-cache-demo
```

## Building blocks

| Concern | Typical host volume name | Container path |
|--------|---------------------------|----------------|
| Flathub system / runtimes | `dockpipe-flatpak-system` | `/var/lib/flatpak` |
| APT package cache | `dockpipe-apt-cache` | `/var/cache/apt` |
| APT lists | `dockpipe-apt-lists` | `/var/lib/apt/lists` |

Override name stems with `DOCKER_DEMO_FLATPAK_VOL`, `DOCKER_DEMO_APT_CACHE_VOL`, `DOCKER_DEMO_APT_LISTS_VOL`.

## dockpipe CLI

`--mount` accepts **named volumes** (same as `docker run -v`):

```bash
dockpipe --mount dockpipe-flatpak-system:/var/lib/flatpak --isolate steam-flatpak -- true
```

Combine with other mounts as needed.

## Scripts

- **`scripts/docker-cache-volumes.sh`** — `docker_cache_volume_ensure` / `docker_cache_volume_append_mounts` for host wrappers.
- **`scripts/docker-steam-flatpak.sh`** — optional `DOCKER_STEAM_FLATPAK_CACHE_VOLUME` (default mounts `dockpipe-flatpak-system:/var/lib/flatpak`); set `DOCKER_STEAM_USE_FLATPAK_CACHE_VOLUME=0` to skip.

## Isolation

“Isolation” here means **separate container filesystem + optional volume mounts**, not a security boundary. For stronger sandboxing, use **Flatpak/firejail on the host**, separate users, or dedicated VMs — see **`sandbox-steam`** workflow notes.
