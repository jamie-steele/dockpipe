# Pipeon Desktop

First-party Tauri desktop shell for Pipeon.

Purpose:
- open the local Pipeon/code-server surface in a dedicated desktop window
- keep Pipeon branding, icon, and window chrome under our control
- avoid the default-browser-tab flow for the Pipeon product surface

This app is intentionally thin. It does not replace DorkPipe, DockPipe, or the
Pipeon web/editor surface. It is just the desktop host window.

Current host affordances:
- native window/icon/chrome
- native clipboard bridge for the embedded Pipeon/code-server surface
- file-picker behavior still depends on the browser/editor flow inside code-server

## Update boundary

`pipeon-desktop` updates the desktop shell only.

It does **not** update:

- the Pipeon code-server/editor surface
- the Pipeon VS Code extension / VSIX
- stock host VS Code or Cursor installs
- unrelated DockPipe / DorkPipe binaries

### Desktop updater configuration

The updater stays inactive unless a release feed is configured at runtime.

Environment variables:

- `PIPEON_DESKTOP_UPDATER_PUBKEY` — Tauri updater public key content
- `PIPEON_DESKTOP_UPDATER_ENDPOINTS` — comma- or newline-separated updater endpoints
- `PIPEON_DESKTOP_UPDATER_ENDPOINT` — single-endpoint fallback
- `PIPEON_DESKTOP_AUTO_UPDATE=0` — disable automatic startup checks

If those values are absent, `pipeon-desktop` runs normally with no updater activity.

### What still refreshes separately

The Pipeon editor surface is handled by `pipeon-dev-stack`, which refreshes the
Pipeon-managed code-server image when its packaged inputs change.

## Build

From the repo root:

```bash
packages/pipeon/assets/scripts/build.sh desktop
```

That writes the canonical binary to:

`packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop`

On Windows hosts the built binary is:

`packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop.exe`

To install a desktop entry and icon for Linux app launchers (for example Pop OS / GNOME):

```bash
packages/pipeon/assets/scripts/build.sh install-desktop-global
```

## Run directly

```bash
PIPEON_URL=http://127.0.0.1:38421/ \
PIPEON_WINDOW_TITLE=Pipeon \
./packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop
```

The Pipeon dev stack uses this automatically when present.
