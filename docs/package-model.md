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

## 3. Project-local state (`.dockpipe/`) and isolation

**`.dockpipe/`** is the **project-local** tree for generated state: host run records (**`.dockpipe/runs/`**), step outputs (**`.dockpipe/outputs.env`**), handoffs, optional demo stubs, and **installed package material** under **`.dockpipe/internal/`**. That keeps **transient and tool-owned** files out of **`templates/`** and the repo root.

**`.dockpipe/internal/packages/`** is the default store for **fetched or compiled** package trees (workflows, core slices, assets) — the same conceptual layout whether content arrived as a **`.tar.gz`** or is produced by a future **`dockpipe package compile`**-style step. **Uncompressed** authoring under **`templates/`** remains normal; a compile step would **validate** workflow YAML, run linters, and **materialize** into **`.dockpipe/internal/...`** so resolution order can stay predictable (**project `templates/`** → **installed packages** → **bundle**).

**Publish outputs** (templates-core tarball, checksums, GitHub release binaries in CI) live under **`release/artifacts/`** (gitignored), not the project **`templates/`** tree — see **`release/README.md`**.

**Direction:** stronger **validation** (schema, lint) at compile/publish time; **single** on-disk layout for “what the runner sees” under **`.dockpipe/internal/`** when you opt into packaged workflows.

## Resolution order (directional)

When fully wired, workflow and profile resolution will **prefer** project **`templates/`**, then **installed** **`.dockpipe/internal/packages/`**, then **embedded** / materialized bundle — same four concepts (**templates**, **runtimes**, **resolvers**, **strategies**), extended by **packages** from the store.

See also **[architecture-model.md](architecture-model.md)** and **[cli-reference.md](cli-reference.md)** (`dockpipe package`, `dockpipe install`).
