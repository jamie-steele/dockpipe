# Package model (store vs working tree)

DockPipe distinguishes **two sides** so pipelines stay clear:

## 1. Packages (installed, store-backed)

**Packages** are **self-contained** artifacts you fetch from an object store (e.g. **Cloudflare R2** behind HTTPS) or another registry. They are **building blocks** for YAML workflows: full workflows, slices of **`templates/core`** (resolvers, runtimes, strategies, assets), or asset packs.

- **Default layout on disk:** **`<workdir>/.dockpipe/internal/packages/`** — kept under **`internal/`** so user-created and installable packages stay separate from other **`.dockpipe/`** state (runs, handoffs, CI).  
  Override with **`DOCKPIPE_PACKAGES_ROOT`** (absolute path, or relative to workdir), e.g. **`vendor/dockpipe-packages`** if you want packages **versioned in git** without fighting a blanket **`.dockpipe/`** ignore.

Suggested subdirectories (mirror authoring concepts; not all are required):

| Path | Role |
|------|------|
| **`.dockpipe/internal/packages/workflows/<name>/`** | Workflow-shaped trees (`config.yml`, steps, …). |
| **`.dockpipe/internal/packages/core/`** | Same **category** rules as **`templates/core/`**: **`resolvers/`**, **`runtimes/`**, **`strategies/`**, **`assets/`**, **`bundles/`**. |
| **`.dockpipe/internal/packages/assets/`** | Optional top-level packs (e.g. large binaries) that are not folded under **`core/`**. |

**Metadata:** each installable unit should include **`package.yml`** next to its payload (see **`dockpipe package manifest`**). Fields include **`name`**, **`version`**, **`title`**, **`description`**, **`author`**, **`website`**, **`license`**, optional **`kind`** (`workflow` \| `core` \| `assets` \| `bundle`).

**Compression:** store objects are typically **`.tar.gz`** (or **`.tar.zst`** later) to keep bandwidth and R2 storage small; the CLI unpacks into the layout above. **Binary-only** packs are possible for asset-only packages if you add a small unpack step later.

**CLI today:** **`dockpipe install core`** pulls **`templates/core`** into **`templates/core`** (bootstrap path). Future **`dockpipe install package …`** will unpack into **`.dockpipe/internal/packages/...`** and wire resolution order (project **`templates/`** + **installed packages** + embedded bundle).

## 2. Uncompressed working tree (authoring / clone)

**Not** “packages” in this sense: the repo you edit every day.

- **`templates/`**, **`src/templates/`**, **`shipyard/workflows/`** — YAML and assets you **build or clone**, commit, and run with **`--workflow`** as today.
- No **`package.yml`** required; this is normal development.

## Resolution order (directional)

When fully wired, workflow and profile resolution will **prefer** project **`templates/`**, then **installed** **`.dockpipe/internal/packages/`**, then **embedded** / materialized bundle — same four concepts (**templates**, **runtimes**, **resolvers**, **strategies**), extended by **packages** from the store.

See also **[architecture-model.md](architecture-model.md)** and **[cli-reference.md](cli-reference.md)** (`dockpipe package`, `dockpipe install`).
