# Release assets

Everything needed to **ship**, **package**, and **document** dockpipe releases lives under **`release/`**:

| Path | Contents |
|------|----------|
| **`packaging/`** | `.deb` builders, Windows **MSI**/WiX, **`install.ps1`**, Homebrew formula stub, winget notes |
| **`releasenotes/`** | Per-version release bodies (`X.Y.Z.md`) consumed by **`.github/workflows/release.yml`** |
| **`docs/`** | Maintainer docs: branching, releasing, dev.to, blog drafts |
| **`demo/`** | Terminal demo recordings / GIFs (`make demo-record`) |
| **`artifacts/`** | *(gitignored)* Local and CI outputs: **`make package-templates-core`**, GitHub release tarballs/debs, **`dockpipe package build core`**. Tracked **`release/`** docs and scripts stay outside this folder. |

User-facing install overview stays in repo-root **`docs/install.md`**.

## Optional R2 mirror

The release workflow can also publish the self-hosted package mirror to a
Cloudflare R2 / S3-compatible bucket after the GitHub release succeeds.

What it publishes from **`release/artifacts/`**:

- **`templates-core-<version>.tar.gz`** + **`install-manifest.json`**
- compiled package-store tarballs from **`dockpipe package build store`**
- **`packages-store-manifest.json`**
- per-file **`.sha256`** files and **`SHA256SUMS.txt`**

Enable it with GitHub Actions configuration:

- repo variable **`R2_PUBLISH=true`**
- repo variable **`DOCKPIPE_RELEASE_BUCKET`** or **`R2_BUCKET`**
- optional repo variables **`R2_PREFIX`**, **`R2_ENDPOINT_URL`**,
  **`AWS_ENDPOINT_URL_S3`**, **`AWS_REGION`**, **`CLOUDFLARE_ACCOUNT_ID`**,
  **`R2_ACCOUNT_ID`**
- repo secrets **`AWS_ACCESS_KEY_ID`** and **`AWS_SECRET_ACCESS_KEY`**

The workflow uses **`dockpipe release upload`** for each artifact, so the
mirror path dogfoods the same S3/R2 upload contract documented in
**`docs/cli-reference.md`**.
