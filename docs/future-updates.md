# Future updates / ideas

Backlog and brainstorm. No commitment to implement in any order.

Only include items that are **not implemented yet**.

**Clarification (so this file isn’t confused with what already shipped):**

- **In-repo / CI today:** Repo-root **`VERSION`** + **`releasenotes/X.Y.Z.md`**; **PRs to `main`/`master`** run **CI** (**`govulncheck`**, **`gosec`**, `go test`, **`make`**, **`.deb`**, **`tests/run_tests.sh`**, **`tests/integration-tests/run.sh`**, release-notes gate) and **CodeQL** (`security-extended`); **merges to `main`/`master`** run **Release** (build + GitHub Release `v$VERSION`). Optional **dev.to** article update when **`DEVTO_PUBLISH`** / **`DEVTO_ARTICLE_ID`** / **`DEVTO_API_KEY`** are set — **[docs/devto.md](devto.md)**. `packaging/homebrew/dockpipe.rb`, **`.github/workflows/release.yml`**, Linux/macOS/Windows artifacts + `.deb` + **Windows `.msi`** + zip, **`packaging/windows/install.ps1`**, **`dockpipe windows setup` / `windows doctor`**, **`dockpipe.exe` → WSL forwarding**, **`packaging/winget/README.md`**, docs (`docs/wsl-windows.md`, `docs/releasing.md`, `docs/branching.md`).
- **Still future:** everything listed below — e.g. **`dockpipe workflow validate`**, **template-level host actions**, **`dockpipe macos doctor`**, **`winget install` from the default Microsoft catalog** (requires a merged `winget-pkgs` PR per release), and **automating Homebrew tap bumps**.

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

**Today:** WSL→Windows bundle fetch is documented (`docs/wsl-windows.md`); `windows setup` sets host-aware env in WSL (`DOCKPIPE_WINDOWS_HOST`, etc.). That is **not** the same as a host-actions feature in workflows.

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

*Add new ideas below.*
