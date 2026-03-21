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

## Workflow UX (remaining)

- Optional: treat captured **stdout** as **`outputs:`**-style env for the next step (today **`capture_stdout`** only writes a host file; it does not merge into the dotenv chain).

---

## Host actions (Windows/macOS/Linux)

**Today:** Native Windows **`dockpipe.exe`** uses host git/docker (same model as Linux/macOS). The **optional WSL bridge** (**`DOCKPIPE_USE_WSL_BRIDGE=1`**, **`dockpipe windows setup`**) forwards commands into a distro — see **[install.md](install.md)** / **[cli-reference.md](cli-reference.md)**. That is **not** the same as first-class **host actions** in workflows.

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

## Optional GUI / web shell (research — not on the core roadmap)

Dockpipe’s product is **run → isolate → act** with Docker and host git — not a replacement IDE or compositor.

If someone wanted a **thin host UI** around a containerized dev server (e.g. port-forward + native webview instead of a full browser tab), that would be a **separate experiment**: no commitment to X11, Wayland, VNC, or WSLg passthrough; those were old brainstorm bullets, not a plan. Anything serious would need a dedicated design and owner.

---

## Terraform & cloud

**Terraform**

- Optional integration: e.g. provider stubs, generated workflow modules, or documented patterns for running Dockpipe-backed automation from Terraform-managed infra (TBD).

**Cloud actions**

- Deeper integrations with hosted CI (beyond “run Dockpipe in GitHub Actions”): reusable **orb** / composite actions, documented secrets patterns, and optional **cloud-side** workflow triggers aligned with **host actions** above.

---

*Add new ideas above the closing sections or extend the sections in place.*
