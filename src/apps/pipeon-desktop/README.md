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

## Build

From the repo root:

```bash
make build-pipeon-desktop
```

That writes the canonical binary to:

`src/apps/pipeon-desktop/bin/pipeon-desktop`

To install a desktop entry and icon for Linux app launchers (for example Pop OS / GNOME):

```bash
make install-pipeon-desktop-global
```

## Run directly

```bash
PIPEON_URL=http://127.0.0.1:38421/ \
PIPEON_WINDOW_TITLE=Pipeon \
./src/apps/pipeon-desktop/bin/pipeon-desktop
```

The Pipeon dev stack uses this automatically when present.
