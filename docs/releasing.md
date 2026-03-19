# Releasing dockpipe

This repo now supports an automated GitHub Actions release pipeline.

**Optional dev.to:** After a release, you can auto-update a dev.to article via the API — see **[devto.md](devto.md)** (GitHub **variables** + **`DEVTO_API_KEY`** secret).

**Ship model:** Merging to **`main`** / **`master`** runs **Release** (see **[branching.md](branching.md)**). Version = repo-root **`VERSION`**; **`releasenotes/X.Y.Z.md`** must exist and be updated on the PR. PRs also run **CI** (tests + release-notes gate).

---

## Release workflow

Pipeline file: `.github/workflows/release.yml`

Trigger options:

1. **Merge (push) to `main` or `master`** — ships **`v$(cat VERSION)`** if **`releasenotes/${VERSION}.md`** exists on that commit.
2. **Manual dispatch** (Actions UI):
   - `version`: optional — defaults to **`VERSION`** on the checked-out branch
   - `dry_run`: `true` → build + artifact upload only, **no** GitHub Release

---

## What the pipeline does

1. Runs `go test ./...`
2. Builds artifacts:
   - `dockpipe_<version>_linux_amd64.tar.gz`
   - `dockpipe_<version>_linux_arm64.tar.gz`
   - `dockpipe_<version>_darwin_amd64.tar.gz`
   - `dockpipe_<version>_darwin_arm64.tar.gz`
   - `dockpipe_<version>_windows_amd64.zip`
   - `dockpipe_<version>_windows_amd64.msi` (WiX, Windows runner)
   - `dockpipe_<version>_amd64.deb`
   - `dockpipe_<version>_arm64.deb`
3. Generates `SHA256SUMS.txt`
4. Uses `releasenotes/<version>.md` as GitHub release body
5. Creates GitHub release and uploads artifacts
6. If `dry_run=true`, uploads artifacts as workflow artifacts and skips release creation

> `releasenotes/<version>.md` is required. The workflow fails fast if it is missing.

**Before merging to `main` (optional but recommended):** run the relevant **[manual QA](manual-qa.md)** pages — at minimum **[manual-qa-core.md](manual-qa-core.md)** for `.deb`; **[manual-qa-windows.md](manual-qa-windows.md)** if you touched the bridge, **MSI**, or `windows setup`; **[manual-qa-macos.md](manual-qa-macos.md)** if you changed Darwin builds or docs.

**winget:** after the release is live, optionally submit/update a manifest for the Microsoft community repo — see **[packaging/winget/README.md](../packaging/winget/README.md)**.

---

## Homebrew release follow-up

After release artifacts are published, update Homebrew formula SHA/version:

- Formula source in repo: `packaging/homebrew/dockpipe.rb`
- Maintainer instructions: `packaging/homebrew/README.md`

---

## dev.to announcement (optional)

If **`DEVTO_PUBLISH=true`** and **`DEVTO_ARTICLE_ID`** are set (plus **`DEVTO_API_KEY`** secret), the Release workflow updates your existing dev.to post. See **[devto.md](devto.md)**.
