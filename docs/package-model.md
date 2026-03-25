# Package model (store vs working tree)

DockPipe distinguishes **two sides** so pipelines stay clear: **what you author** in a repo, and **what you install** as versioned packages. This doc also records the **intended** compile / package / release pipeline (some steps are still evolving in the CLI).

## Official reference vs repo-local trees

**Downstream and the engine** do **not** depend on **this repository’s** **`.staging/`** or repo-root **`workflows/`** — those are **maintainer-only** (CI, dogfood, experiments). They **must not** be required for a minimal install or for **`dockpipe`** semantics.

**Canonical** material for consumers is **published** artifacts: **`dockpipe install core`**, **`dockpipe package compile` → package → release**, and **HTTPS/static origins** you operate (e.g. **`core.*` / `dockpipe.*`** namespaces once live). **Pin installs and docs to those origins**, not to mutable paths in a checkout. That keeps packages **self-contained** (bounded YAML + assets + declared deps) so they **cannot** change **`src/lib/`** or **`src/cmd/`** without a **separate** engine release.

## Authoring vs execution (two modes, both supported)

| Mode | What you run | Friction | Notes |
|------|----------------|----------|--------|
| **Source / today** | Workflow YAML from **`workflows/`**, legacy **`templates/<name>/`**, etc. | **Low** for day-to-day editing — no compile step required. | **`scripts/…`** resolves per **`paths.go`** (project **`scripts/`** first, then bundled **resolvers** / **bundles** / **`assets/scripts/`**). Users can keep scripts wherever those rules allow. |
| **Compiled / packaged** | **`packages/workflows/<name>/`**, **`packages/resolvers/<name>/`**, plus **`packages/core/`** (spine only), from **`compile all`** under **`.dockpipe/internal/packages/`**. Bundles are optional (**`compile bundles`** or **`compile all --with-bundles`**). | **One** compile (or CI) before run. | **Cleaner** tree: optional **`package.yml`** per slice; resolver search prefers **`packages/resolvers/`** when present. |

**Authors are not forced to pick one path:** keep editing and running from source for low friction; use **compile → package → release** when you want a **self-contained** published artifact.

### Local compile (`dockpipe build` / `dockpipe package compile` / `dockpipe compile`)

Materialize a **project-local** store under **`.dockpipe/internal/packages/`** without moving authoring trees.

**Repo-root `dockpipe.config.json` (optional)** — JSON with a **`compile`** object:

- **`workflows`** — array of repo-relative (or absolute) **directories** to scan for named workflow folders (each with **`config.yml`**).
- **`resolvers`** — array of **directories** whose **children** are resolver profile dirs to merge into **`packages/resolvers/<name>/`** (later roots overlay earlier names).
- **`bundles`** — same for **`packages/bundles/<name>/`**.
- **`core_from`** — optional path override for **`compile core`** (same as **`--from`**).
- **`secrets`** (optional) — not secrets themselves; pointers for humans and tooling:
  - **`op_inject_template`** — repo-relative or absolute path to a **mapping file** for **`op inject`** (e.g. **`.env.op.template`** with **`op://`** lines). **`dockpipe doctor`** reports whether that file exists when **`dockpipe.config.json`** is present in the current directory.
  - **`notes`** — free-text reminder (e.g. vault naming, policy).

If **`dockpipe.config.json`** is **missing**, compile uses built-in defaults. If a key is **omitted**, defaults apply for that slice; **`dockpipe init`** seeds a starter JSON. **`--no-staging`** filters out paths under **`.staging/`** when resolving defaults or config lists.

Compile steps:

1. **`compile core`** — copies **`src/core`** (default when **`src/core/runtimes`** exists) or **`templates/core`**, or **`compile.core_from`** / **`--from`**, into **`packages/core/`** and writes **`package.yml`** (`kind: core`). **Omits** top-level **`resolvers/`**, **`bundles/`**, and **`workflows/`** from that copy so those slices stay separate packages.
2. **`compile resolvers`** — repeatable **`--from`**; defaults merge **`src/core/resolvers`**, **`templates/core/resolvers`**, then **`.staging/resolvers`** (each path must exist) into **`packages/resolvers/<name>/`**.
3. **`compile bundles`** — repeatable **`--from`**; defaults from config or **`.staging/bundles`** into **`packages/bundles/<name>/`**.
4. **`compile workflows`** — every subdir with **`config.yml`** under each **`--from`** root; defaults **`workflows/`** then **`.staging/workflows/`** (or **`dockpipe.config.json`**).
5. **`compile all`** (alias: **`dockpipe build`**) — runs **core → resolvers → workflows**. Bundles only when **`--with-bundles`** is set (otherwise use **`compile bundles`**). **`dockpipe clean`** removes the compiled store; **`dockpipe rebuild`** runs **clean** then **build**.

The runner checks **compiled `packages/resolvers/`** and **`packages/core/`** before **`.staging`** and authoring **`CoreDir`** so you can **opt in** to the compiled store per workdir. Edit **`package.yml`** after compile to add **namespaces**, **`depends`**, and metadata for store-shaped workflows.

## Network boundary: install, not every `run`

**HTTPS / CDN / registry traffic** should be confined to **explicit install (and publish)** commands — e.g. **`dockpipe install core`**, future **`dockpipe install package …`**, **`dockpipe release upload`**. After artifacts are on disk, **`dockpipe run`** against local workflows or installed packages should **not** need network unless the **workflow itself** does (e.g. `docker pull`, API calls).

## Lifecycle: compile → package → release

1. **`compile`** — Validate workflow YAML and **materialize** a **self-contained** tree: copy the workflow and (as the implementation grows) **pull in domain-specific assets** referenced from source so the compiled directory is the **single** execution root for that package.
2. **`package`** — Archive **that compiled tree** (plus **`package.yml`**, checksums, optional lock metadata). **`dockpipe package build store`** turns the compiled store into **gzip tarballs** (one per core / workflow / resolver; bundles only with **`--only bundles`**) and **`packages-store-manifest.json`**. **`dockpipe run --workflow`** can **stream** a workflow from **`dockpipe-workflow-<name>-*.tar.gz`** when **`config.yml` in the archive** sets **`namespace:`** and on-disk paths do not win (see **`packages.tarball_dir`** / **`packages.namespace`** in **`dockpipe.config.json`**). **`dockpipe package build core`** builds the **`templates/core`** artifact for **`dockpipe install core`**.
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
| **`.dockpipe/internal/packages/core/`** | Compiled **spine** only: **`runtimes/`**, **`strategies/`**, **`assets/`**, etc. — not resolver/bundle/workflow packages. |
| **`.dockpipe/internal/packages/resolvers/<name>/`** | One resolver package per profile (same shape as **`templates/core/resolvers/<name>/`**). |
| **`.dockpipe/internal/packages/bundles/<name>/`** | One bundle package per domain. |
| **`.dockpipe/internal/packages/assets/`** | Optional top-level packs (e.g. large binaries) that are not folded under **`core/`**. |

**Metadata:** each installable unit should include **`package.yml`** next to its payload (see **`dockpipe package manifest`**). Core fields: **`name`**, **`version`**, **`title`**, **`description`**, **`author`**, **`website`**, **`license`**, **`kind`** (`workflow` \| `resolver` \| `core` \| `assets` \| `bundle`). Optional **`namespace`** — same rules as workflow **`config.yml`** **`namespace:`** (lowercase label; reserved words like **`dockpipe`**, **`core`**, **`system`** are rejected).

**Rich metadata (authoring & store discovery)** — optional but recommended for **workflow** and **resolver** packages:

| Field | Purpose |
|-------|---------|
| **`tags`**, **`keywords`** | Faceted search / UI filters |
| **`min_dockpipe_version`** | Semver constraint on the CLI |
| **`repository`** | Source repo URL |
| **`provides`** | Resolver capability names (e.g. tool ids) for **`kind: resolver`** |
| **`requires_resolvers`** | Hint compatible resolver profiles for **`kind: workflow`** |
| **`depends`** | Other package **names** this package expects |
| **`namespace`** | Author/org label for discovery and future namespaced installs (validated; see **`domain.ValidateNamespace`**) |
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
