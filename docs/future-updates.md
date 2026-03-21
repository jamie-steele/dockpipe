# Future updates / ideas

Backlog and brainstorm. **No commitment** to implement in any order.

Only include items that are **not implemented yet**.

**Clarification (so this file isn’t confused with what already shipped):**

- **Today:** Integrate on **`staging`**; when ready, **PR `staging` → `master`** ships (**`VERSION`** + **`releasenotes/X.Y.Z.md`**). Details: **[releases/branching.md](releases/branching.md)**, **[releases/releasing.md](releases/releasing.md)**.
- **This file** lists **future** ideas only — not the current CI matrix, release artifacts, or shipped features.

---

## Windows MSI (release pipeline)

- **Per-user MSI** as a normal release asset (WiX), without an opt-in marker file — or keep opt-in but make CI **reliable** and documented.
- **`winget install`** from the default Microsoft catalog (requires a merged **`winget-pkgs`** PR per release) — see **[packaging/winget/README.md](../packaging/winget/README.md)**.

---

## Workflow UX enhancements

- Optional repo-root `dockpipe.yml` (same shape as template config).
- `dockpipe init` scaffolds richer default config.
- JSON schema validation / linting for workflow YAML (e.g. `dockpipe workflow validate`).
- Partial config imports / sharing snippets across templates.
- Optional stdout capture as `outputs` for step chaining.
- Optional small JSON manifest output format for steps.

---

## Host actions (Windows/macOS/Linux)

**Today:** Native Windows `dockpipe.exe` uses host git/docker; WSL→Windows bundle fetch is documented in **[wsl-windows.md](wsl-windows.md)**; `dockpipe windows setup` applies when using the **optional** bridge. That is **not** the same as first-class **host actions** in workflows.

**Still to build:**

- Built-in host actions (open URL/path/app, fetch/apply results) as first-class commands or workflow hooks.
- User-defined host scripts/commands with an explicit, documented contract.
- Template-level `host-actions` (or equivalent) in YAML.
- Optional allowlist/permission model for stricter environments.
- Cross-platform action abstraction (Windows/macOS/Linux).

---

## macOS distribution and DX

**Today:** macOS tarballs ship from the release workflow; a Homebrew **formula source** lives in-repo (`packaging/homebrew/dockpipe.rb`). Users still need a **published tap** that tracks those releases.

**Still to build or automate:**

- Automate or standardize **tap repo updates** each release (bump `url` / `sha256` in `Formula/dockpipe.rb` — see `packaging/homebrew/README.md`).
- Optional **codesign / notarize** macOS binaries (releases are currently unsigned archives).
- **`dockpipe macos doctor`** (Docker, bash, PATH sanity checks).

---

## Isolated GUI & IDE workbenches

Run graphical applications (IDEs, browsers, AI agents) in a disposable container with a native host-side experience. The core **run → isolate → act** flow stays: work in the window, close to exit, and trigger host-side actions (e.g. git commits) automatically.

### The “lightweight” approach (Electron / web stacks)

Most modern dev tools (VS Code, Cursor, Claude Code, etc.) sit on web engines — decouple the **engine** from the **view** to cut bloat.

- **Isolate (container):** headless backend (e.g. code-server or a specialized AI dev server).
- **View (host):** a small **Go** front-end using native OS webviews (**WebKitGTK** / **WebView2**) pointed at the container’s mapped port.

**Win:** near-native performance, OS clipboard and shortcuts, low-latency rendering, without shipping a ~500MB Chromium shell.

### Legacy GUI support (X11 / Wayland)

For heavier or non-web apps (traditional IDEs, browsers, arbitrary GUI tools):

- **X11:** headless VNC inside the container + **noVNC** in the Go webview — pixel-accurate isolation for untrusted GUI workloads on X11 hosts (e.g. Pop!_OS).
- **Wayland:** passthrough the compositor socket on Wayland-native hosts, or **WSLg** on Windows, for hardware-accelerated, high-DPI display while keeping workload isolation.

### Native lifecycle & shell

- **Lifecycle sync:** closing the Go webview stops the container and moves the workflow into the **act** phase (audit complete).
- **Universal frame:** a standard Dockpipe chrome (commit, status, revert, etc.) regardless of what runs inside the webview.

---

## Terraform & cloud

**Terraform**

- Optional integration: e.g. provider stubs, generated workflow modules, or documented patterns for running Dockpipe-backed automation from Terraform-managed infra (TBD).

**Cloud actions**

- Deeper integrations with hosted CI (beyond “run Dockpipe in GitHub Actions”): reusable **orb** / composite actions, documented secrets patterns, and optional **cloud-side** workflow triggers aligned with **host actions** above.

---

*Add new ideas above the closing sections or extend the sections in place.*
