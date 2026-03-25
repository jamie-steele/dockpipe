# Package model (store vs working tree)

DockPipe distinguishes **two sides** so pipelines stay clear: **what you author** in a repo, and **what you install** as versioned packages. This doc also records the **intended** compile / package / release pipeline (some steps are still evolving in the CLI).

## Official reference vs repo-local trees

**Downstream and the engine** do **not** depend on **this repository’s** **`.staging/`** or repo-root **`workflows/`** — those are **maintainer-only** (CI, dogfood, experiments). They **must not** be required for a minimal install or for **`dockpipe`** semantics.

**Canonical** material for consumers is **published** artifacts: **`dockpipe install core`**, **`dockpipe package compile` → package → release**, and **HTTPS/static origins** you operate (e.g. **`core.*` / `dockpipe.*`** namespaces once live). **Pin installs and docs to those origins**, not to mutable paths in a checkout. That keeps packages **self-contained** (bounded YAML + assets + declared deps) so they **cannot** change **`src/lib/`** or **`src/cmd/`** without a **separate** engine release.

## Authoring vs execution (two modes, both supported)

| Mode | What you run | Friction | Notes |
|------|----------------|----------|--------|
| **Source / today** | Workflow YAML from **`workflows/`**, legacy **`templates/<name>/`**, etc. | **Low** for day-to-day editing — no compile step required. | **`scripts/…`** resolves per **`paths.go`** (project **`scripts/`** first, then bundled **resolvers** / **bundles** / **`assets/scripts/`**). Users can keep scripts wherever those rules allow. |
| **Compiled / packaged** | Workflow from **`.dockpipe/internal/packages/workflows/<name>/`** (material produced by **`dockpipe package compile workflow`** today; richer “grab referenced assets” compile later). | **One** compile (or CI) before run. | **Cleaner** tree: domain-specific files can be **co-located** under the compiled root so the runner does **less path search** at **`run`** time (often faster startup; heavy steps like Docker still dominate many workflows). |

**Authors are not forced to pick one path:** keep editing and running from source for low friction; use **compile → package → release** when you want a **self-contained** published artifact.

## Network boundary: install, not every `run`

**HTTPS / CDN / registry traffic** should be confined to **explicit install (and publish)** commands — e.g. **`dockpipe install core`**, future **`dockpipe install package …`**, **`dockpipe release upload`**. After artifacts are on disk, **`dockpipe run`** against local workflows or installed packages should **not** need network unless the **workflow itself** does (e.g. `docker pull`, API calls).

## Lifecycle: compile → package → release

1. **`compile`** — Validate workflow YAML and **materialize** a **self-contained** tree: copy the workflow and (as the implementation grows) **pull in domain-specific assets** referenced from source so the compiled directory is the **single** execution root for that package.
2. **`package`** — Archive **that compiled tree** (plus **`package.yml`**, checksums, optional lock metadata). Packaging stays simple because compile already assembled the payload.
3. **`release`** — Upload tarball + manifest to your **static origin**; consumers **`install`** to pull it.

**CI** can chain the same steps locally: **compile → package → release**.

## Workflow install and resolver dependencies

When **`dockpipe install`** (workflow package) exists end-to-end, installing a **workflow** should **also install declared dependencies**, primarily **`kind: resolver`** packages (**`depends`** / pins in **`package.yml`**), including a **transitive** closure where needed. **Domain-specific** scripts and assets belong **inside** the workflow package / compile output — not as a separate CDN hop for every file at run time. **Resolver** packages remain **shared adapters** (tool profiles, resolver-owned assets).

## Distribution split (repo vs store)

| What | Where it usually lives | Notes |
|------|-------------------------|--------|
| **Runtimes** | **Repo / bundled `templates/core/runtimes/`** | Stable, light profiles — **not** the main “store” surface. |
| **Strategies** | **Repo / bundled `templates/core/strategies/`** | Thin lifecycle wiring — same as runtimes: **keep in tree**. |
| **Compiled core** | **HTTPS/S3 (e.g. R2)** + **`dockpipe install core`** | **Tight `templates/core` tarball** so installs stay small; refresh without cloning the whole upstream repo. |
| **Resolvers** | **Bundled** and/or **store packages** | **Plugin adapters** — shared across workflows; extended catalogs ship as packages with rich **`package.yml`**. |
| **Workflows** | **Authoring tree**, **`.dockpipe/internal/packages/workflows/`**, or **store** | **Primary rich-metadata** packages for authoring and discovery (`kind: workflow`). |

**Mental model:** the **CLI + slim core** in git or from S3 gives you a **lightweight spine**; **workflows** and **resolver** packs are what you **browse, version, and install** from a registry or internal bucket (the “plugin store” layer).

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

**Metadata:** each installable unit should include **`package.yml`** next to its payload (see **`dockpipe package manifest`**). Core fields: **`name`**, **`version`**, **`title`**, **`description`**, **`author`**, **`website`**, **`license`**, **`kind`** (`workflow` \| `resolver` \| `core` \| `assets` \| `bundle`).

**Rich metadata (authoring & store discovery)** — optional but recommended for **workflow** and **resolver** packages:

| Field | Purpose |
|-------|---------|
| **`tags`**, **`keywords`** | Faceted search / UI filters |
| **`min_dockpipe_version`** | Semver constraint on the CLI |
| **`repository`** | Source repo URL |
| **`provides`** | Resolver capability names (e.g. tool ids) for **`kind: resolver`** |
| **`requires_resolvers`** | Hint compatible resolver profiles for **`kind: workflow`** |
| **`depends`** | Other package **names** this package expects |
| **`allow_clone`** | If **`true`**, **`dockpipe clone`** may export the compiled tree to **`workflows/`**; if false or omitted, clone is refused. |
| **`distribution`** | Optional hint: **`source`** or **`binary`** (documentation for store pages). |

The Go type **`domain.PackageManifest`** parses these keys; see **`src/lib/dockpipe/domain/package_manifest.go`**.

**Compression:** store objects are typically **`.tar.gz`** (or **`.tar.zst`** later) to keep bandwidth and R2 storage small; the CLI unpacks into the layout above. **Binary-only** packs are possible for asset-only packages if you add a small unpack step later.

**CLI today:** **`dockpipe install core`** pulls **`templates/core`** into **`templates/core`** (bootstrap path). **`dockpipe package compile workflow`** validates YAML and copies a workflow tree into **`.dockpipe/internal/packages/workflows/<name>/`** (see **`docs/cli-reference.md`**). Future **`dockpipe install package …`** will unpack registry artifacts and extend resolution order (project **`workflows/`** + **installed packages** + embedded bundle).

### Clone, education, and commercial packages

- **`package.yml`** may set **`allow_clone: true`** so **`dockpipe clone <name>`** can copy a compiled workflow package from **`.dockpipe/internal/packages/workflows/<name>/`** into **`workflows/<name>/`** (or **`--to`**). This supports **education** and **forking** when the author opts in.
- **`allow_clone: false`** or omitting **`allow_clone`** means **`dockpipe clone`** refuses — appropriate for **commercial** or **binary-only** releases where the publisher does not grant a recoverable source tree.
- Optional **`distribution: binary`** documents that the published artifact is not meant for source recovery; publishers who need **strong** protection should ship **only** non-recoverable binaries (obfuscation, native modules, etc.) — DockPipe cannot cryptographically enforce that; **`allow_clone`** is the explicit license bit for **`dockpipe clone`**.
- **`dockpipe package compile workflow`** writes **`allow_clone: true`** and **`distribution: source`** into a generated **`package.yml`** so local compiles stay cloneable unless you edit the manifest before release.

## 2. Uncompressed working tree (authoring / clone)

**Not** “packages” in this sense: the repo you edit every day.

- **`workflows/`** (default project root), **`src/core/workflows/<name>/`** (bundled examples in the dockpipe repo), legacy **`templates/<name>/`** — YAML and assets you **build or clone**, commit, and run with **`--workflow`** as today.
- No **`package.yml`** required; this is normal development.

## 3. Project-local state (`.dockpipe/`) and isolation

**`.dockpipe/`** is the **project-local** tree for generated state: host run records (**`.dockpipe/runs/`**), step outputs (**`.dockpipe/outputs.env`**), handoffs, optional demo stubs, and **installed package material** under **`.dockpipe/internal/`**. That keeps **transient and tool-owned** files out of the main authoring trees and the repo root.

**`.dockpipe/internal/packages/`** is the default store for **fetched or compiled** package trees (workflows, core slices, assets) — the same conceptual layout whether content arrived as a **`.tar.gz`** or from **`dockpipe package compile workflow`**. **Uncompressed** authoring under **`workflows/`** remains normal; **compile** validates and **materializes** into **`.dockpipe/internal/...`** when you opt into the packaged path. Resolution order for **`--workflow`** is implemented in **`workflow_dirs.go`** (**`workflows/`** → **packages** → legacy **`templates/`** paths, etc.).

**Publish outputs** (templates-core tarball, checksums, GitHub release binaries in CI) live under **`release/artifacts/`** (gitignored), not the project workflow tree — see **`release/README.md`**.

**Direction:** stronger **validation** (schema, lint) at **compile** time; optional **package** / **install** for store-backed workflows; **source** mode stays available for low-friction authoring.

## Resolution order (directional)

**`--workflow`** resolution (see **`workflow_dirs.go`**) already checks **`.dockpipe/internal/packages/workflows/<name>/`** (after **`workflows/`** and before legacy **`templates/<name>/`**) when **`dockpipe run`** uses **`--workdir`** or the current directory; **`dockpipe doctor`** and **`ResolveWorkflowConfigPath(repoRoot, name)`** without a workdir skip the packages store.

When fully wired end-to-end, workflow name resolution will **prefer** project **`workflows/`**, then **installed** **`.dockpipe/internal/packages/workflows/`**, then legacy **`templates/`** paths and the embedded bundle — same four concepts (**workflow**, **runtime**, **resolver**, **strategy**), extended by **packages** from the store.

See also **[architecture-model.md](architecture-model.md)** and **[cli-reference.md](cli-reference.md)** (`dockpipe package`, `dockpipe install`).
