# Releasing dockpipe

This repo now supports an automated GitHub Actions release pipeline.

**Optional dev.to:** **PUT** a main article (**`DEVTO_ARTICLE_ID`**) and/or **POST** a one-time post per release (**`DEVTO_ONE_TIME_POST`**) — see **[devto.md](devto.md)** (**`DEVTO_PUBLISH`**, **`DEVTO_API_KEY`** secret).

**Ship model:** Integrate on **`staging`**; when ready, **PR `staging` → `master`** — that merge runs **Release** (see **[branching.md](branching.md)**). Version = repo-root **`VERSION`**; **`release/releasenotes/X.Y.Z.md`** must exist and be updated on the **ship** PR. **CI** runs on **`staging`** PRs too (tests only); the **VERSION + release-notes gate** applies only to PRs **into `master`**.

**Release notes body:** Copy **[TEMPLATE.md](../releasenotes/TEMPLATE.md)** to **`release/releasenotes/X.Y.Z.md`**, replace **`X.Y.Z`** / **`vX.Y.Z`**, and fill in **What’s new**. The **Installation** section must include **Linux**, **macOS**, and **Windows** with concrete commands (`.deb` + **`.apk` / `.rpm` / Arch `.pkg.tar.zst`** + **`linux/install.sh`** + tarballs + source, Homebrew + Darwin tarballs + source, `install.ps1` / MSI / zip + optional WSL). That file becomes the GitHub Release description — users should not have to hunt **`docs/install.md`** for basics.

---

## Release workflow

Pipeline file: `.github/workflows/release.yml`

Trigger options:

1. **Merge (push) to `master`** — ships **`v$(cat VERSION)`** if **`release/releasenotes/${VERSION}.md`** exists on that commit.
2. **Manual dispatch** (Actions UI):
   - `version`: optional — defaults to **`VERSION`** on the checked-out branch
   - `dry_run`: `true` → build + artifact upload only, **no** GitHub Release
   - `build_msi`: optional — defaults to **`true`**. On **push** to `master`, MSI is built when the committed marker file **`release/packaging/msi/SHIP_MSI`** is present. This repo currently keeps that marker checked in, so normal releases include WiX/MSI unless you intentionally remove it.

---

## What the pipeline does

1. Runs `go test ./...`
2. Builds artifacts:
   - `dockpipe_<version>_linux_amd64.tar.gz`
   - `dockpipe_<version>_linux_arm64.tar.gz`
   - `dockpipe_<version>_darwin_amd64.tar.gz`
   - `dockpipe_<version>_darwin_arm64.tar.gz`
   - `dockpipe_<version>_windows_amd64.zip`
   - `dockpipe_<version>_windows_amd64.msi` (WiX, Windows runner) — **only if** MSI is enabled (see **`release/packaging/msi/SHIP_MSI`** on push, or **`build_msi`** on manual dispatch)
   - `dockpipe_<version>_amd64.deb`
   - `dockpipe_<version>_arm64.deb`
   - `dockpipe_<version>_linux_amd64.apk`, `…_linux_arm64.apk` (Alpine, via **[build-nfpm.sh](../packaging/build-nfpm.sh)**)
   - `dockpipe_<version>_linux_amd64.rpm`, `…_arm64.rpm` (Fedora/RHEL-compatible)
   - `dockpipe_<version>_linux_amd64.pkg.tar.zst`, `…_arm64.pkg.tar.zst` (Arch Linux)
3. Generates `SHA256SUMS.txt`
4. Uses `release/releasenotes/<version>.md` as GitHub release body (must include **Linux**, **macOS**, and **Windows** install instructions — see **[TEMPLATE.md](../releasenotes/TEMPLATE.md)**)
5. Creates GitHub release and uploads artifacts
6. If `dry_run=true`, uploads artifacts as workflow artifacts and skips release creation

## Pipeon desktop updater boundary

If/when this repo publishes `pipeon-desktop` updater artifacts, keep the updater scope narrow:

- updater artifacts may replace the **Tauri desktop shell** only
- updater artifacts must **not** bundle the Pipeon code-server image, Pipeon VSIX, stock VS Code, Cursor, or unrelated DockPipe/DorkPipe binaries

Treat the Pipeon desktop shell and the Pipeon editor/runtime surface as **separate distribution lanes**:

- **desktop shell** = signed Tauri updater artifacts plus a `latest.json` feed
- **Pipeon surface** = explicit refresh/rebuild/restart flow for the code-server image, VSIX, and local first-party binaries

> `release/releasenotes/<version>.md` is required. The workflow fails fast if it is missing.

**Before merging to `master` (optional but recommended):** run **[manual QA](../../docs/qa/manual-qa.md)** for the platforms you changed.

**winget:** after the release is live, optionally submit/update a manifest for the Microsoft community repo — see **[../packaging/winget/README.md](../packaging/winget/README.md)**.

---

## Homebrew release follow-up

After release artifacts are published, update Homebrew formula SHA/version:

- Formula source in repo: `release/packaging/homebrew/dockpipe.rb`
- Maintainer instructions: `release/packaging/homebrew/README.md`

---

## dev.to announcement (optional)

If **`DEVTO_PUBLISH=true`** and **`DEVTO_API_KEY`** are set, the workflow can **PUT** the main article (**`DEVTO_ARTICLE_ID`**) and/or **POST** a new one-time article each release (**`DEVTO_ONE_TIME_POST=true`**). See **[devto.md](devto.md)**.
