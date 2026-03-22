# `sandbox-steam` workflow

**Purpose:** Small **host-side** helper for thinking about **Steam** (Valve client) alongside **sandbox-style** isolation — without changing dockpipe **core**.

## Run

From a dockpipe checkout (or with `DOCKPIPE_REPO_ROOT` set):

```bash
dockpipe --workflow sandbox-steam --workdir /path/to/your/project
```

Use a **real** project directory; the README path above is only a placeholder.

## What it does

1. Runs `scripts/sandbox-steam.sh` on the **host** (`skip_container: true`).
2. Prints `DOCKPIPE_WORKDIR`, repo root, and which **Steam launcher** it would use (see below).
3. Optionally **`exec`** that launcher when `STEAM_SANDBOX_LAUNCH=1` (blocks until Steam exits).

## Environment (host script)

| Variable | Meaning |
|----------|---------|
| `STEAM_CMD` | Absolute path to the `steam` binary; overrides discovery. |
| `STEAM_USE_FLATPAK=1` | Prefer Flatpak (`STEAM_FLATPAK_APP`) before `steam` on `PATH`. |
| `STEAM_FLATPAK_APP` | Flatpak app id (default `com.valvesoftware.Steam`). |
| `STEAM_SANDBOX_LAUNCH=1` | Run the resolved launcher instead of only printing hints. |

Pass through dockpipe when needed, e.g. `dockpipe --workflow sandbox-steam --env STEAM_USE_FLATPAK=1 --workdir "$PWD"`.

Discovery order when `STEAM_CMD` is unset:

1. If `STEAM_USE_FLATPAK=1` and the Flatpak app is installed → `flatpak run …`
2. Else if `steam` is on `PATH` → that binary
3. Else if the Flatpak app is installed → `flatpak run …`

## Automating Flatpak install (user-facing tools, legally sound pattern)

You can script **installing Flatpak itself** and **adding Flathub** using your distro’s packages and **Flathub’s official remote URL** — that is normal system setup (Flatpak is open-source infrastructure; Flathub is the standard remote).

You can then run **`flatpak install`** with the **official** application id from Flathub (e.g. `com.valvesoftware.Steam`). That path downloads Valve’s client through Flatpak’s usual machinery (user consent, Steam’s terms when they run Steam). Your tool is **not** re-hosting or redistributing Steam binaries; it is orchestrating the **same** install flow a user would run by hand.

Typical building blocks (exact package names differ by distro):

1. Install **`flatpak`** (and on some systems **`xdg-desktop-portal`** / related portal packages) via the system package manager.
2. Add Flathub once:  
   `flatpak remote-add --if-not-exists flathub https://dl.flathub.org/repo/flathub.flatpakrepo`  
   (Use the URL from [Flathub setup](https://flathub.org/) if it changes.)
3. Install Steam:  
   `flatpak install -y flathub com.valvesoftware.Steam`  
   (`-y` assumes non-interactive policy; some setups still prompt for polkit/password — handle that in your UI.)

**Caveat:** Running Flatpak *inside* a minimal container often needs extra privileges, D-Bus, and session bits; many people run Flatpak on the **host** and only orchestrate from there. Dockpipe does not special-case this in core — keep it in your launcher or image.

## Customize

Edit **`scripts/sandbox-steam.sh`** in your fork or overlay — do not need to touch **`lib/dockpipe/`**.

## Pipeon Basic

This workflow sets **`category: app`** so it appears in **Basic** mode alongside other launchable tools. It still only runs the host script (Steam notes / optional launch).
