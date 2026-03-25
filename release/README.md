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
