# Package model (store vs working tree)

DockPipe distinguishes **what you author** (source trees) from **what you ship** (tarballs and the local compile store). This doc describes that split and how **`dockpipe package compile`** / **`dockpipe build`** validate packages.

## Canonical layout (mental model)

DockPipe has **three** package/artifact roots. Do not collapse them into one path.

1. **Project-local build output** lives under **`<workdir>/bin/.dockpipe/`**. The default project package root is **`<workdir>/bin/.dockpipe/internal/packages/`**, or **`DOCKPIPE_PACKAGES_ROOT`** when explicitly overridden. **`dockpipe package compile`** materializes **tarballs** here: **`core/`**, **`resolvers/`**, **`workflows/`** (**`dockpipe-workflow-*`** only). This is a **working** store for the project, not the global install root.

2. **Global installs** live under **`DOCKPIPE_GLOBAL_ROOT`** when set, otherwise the OS data dir from **`GlobalDockpipeDataDir()`** (for example **`~/.local/share/dockpipe`** on Linux). Global packages use **`<global-root>/packages/`** and global core uses **`<global-root>/templates/core/`**. There is no **`bin/`** segment in the global root.

3. **Published / remote packages** are versioned tarballs on a static origin or registry-like store. Each installable unit is a tarball (for example **`dockpipe-workflow-<name>-<ver>.tar.gz`**, **`dockpipe-resolver-…`**, **`dockpipe-core-…`**). The engine can load package tarballs directly when appropriate; it does not need every remote package unpacked into an authoring tree.

4. **Source-level** resolution applies when you point at **authoring** trees: repo **`workflows/`**, **`templates/`**, **`src/core/…`**, etc. — normal edit/run **without** packaging.

5. **Build (`dockpipe build` / `compile all`)** is considered successful only if:
   - **`compile`** order is **core → resolvers → workflows** (legacy **`compile.bundles`** paths are merged into workflow roots),
   - each **workflow** and **resolver** package resolves a **valid `namespace`** (from **`package.yml`**, **`config.yml` / `resolver.yaml`**, or repo-root **`dockpipe.config.json`** **`packages.namespace`** as a default),
   - every **`depends`** entry in **`package.yml`** names a package **already present** in the compiled store (names from **`package.yml`** under **`core/`**, **`resolvers/`**, **`workflows/`**).

**Runtime** may still resolve workflows from **project-local tarballs** under **`bin/.dockpipe/internal/packages/workflows/`**, **global packages** under **`<global-root>/packages/workflows/`**, or **`packages.tarball_dir`** / **`release/artifacts`** when no on-disk workflow config wins; optional **`packages.namespace`** filters tarball choice when set.

## State roots

| Scope | Root | What belongs there |
|------|------|--------------------|
| **Project-local** | **`<workdir>/bin/.dockpipe/`** | Compile outputs, project package store, run records, project-scoped image artifact cache, package-scoped state. |
| **Global** | **`DOCKPIPE_GLOBAL_ROOT`** or **`GlobalDockpipeDataDir()`** | User-wide installed core, resolver/workflow packages, global image artifact metadata, global download cache. |
| **Published remote** | Static origin / package registry / OCI registry | Versioned package tarballs and OCI image refs. |

In Go code, project-local paths must derive from **`infrastructure.DockpipeDirRel`**, **`StateRoot`**, **`PackagesRoot`**, and related helpers. Global paths must derive from **`GlobalDockpipeDataDir`** and the global package/image helpers. Do not spell a bare **`.dockpipe/internal`** path by hand.

## Official reference vs repo-local trees

**Downstream and the engine** do **not** depend on **this repository’s** **`.staging/`** or repo-root **`workflows/`** — those are **maintainer-only** (CI, dogfood, experiments). They **must not** be required for a minimal install or for **`dockpipe`** semantics.

**Canonical** material for consumers is **published** artifacts: **`dockpipe install core`**, **`dockpipe package compile` → package → release**, and **HTTPS/static origins** you operate (e.g. **`core.*` / `dockpipe.*`** namespaces once live). **Pin installs and docs to those origins**, not to mutable paths in a checkout. That keeps packages **self-contained** (bounded YAML + assets + declared deps) so they **cannot** change **`src/lib/`** or **`src/cmd/`** without a **separate** engine release.

## Authoring vs execution (two modes, both supported)

| Mode | What you run | Friction | Notes |
|------|----------------|----------|--------|
| **Source / today** | Workflow YAML from **`workflows/`**, legacy **`templates/<name>/`**, etc. | **Low** for day-to-day editing — no compile step required. | **`scripts/…`** resolves per **`paths.go`** (project **`scripts/`** first, then bundled **resolvers** / **bundles** / **`assets/scripts/`**). Users can keep scripts wherever those rules allow. |
| **Compiled / packaged** | **`packages/workflows/`** (tarballs), **`packages/resolvers/`**, **`packages/core/`**, from **`compile all`** under **`bin/.dockpipe/internal/packages/`**. | **One** compile (or CI) before run. | **Cleaner** tree: optional **`package.yml`** per slice; resolver search prefers **`packages/resolvers/`** when present. |
| **Global installed** | **`<global-root>/packages/workflows/`**, **`<global-root>/packages/resolvers/`**, **`<global-root>/templates/core/`**. | **One install/update** per user or machine. | Shared extensions available to many projects without copying them into each repo. |

**Authors are not forced to pick one path:** keep editing and running from source for low friction; use **compile → package → release** when you want a **self-contained** published artifact.

### Authoring: workflow YAML vs resolver / runtime / strategy slices

A **workflow** is primarily **`config.yml`** (plus assets next to it). In that file you **declare** what to run and **which profiles** to merge — you do **not** embed full resolver or strategy trees inside the workflow directory (unless you choose colocated assets for scripts/images).

| Concern | Where definitions usually live | What the workflow YAML does |
|--------|--------------------------------|-----------------------------|
| **Runtime** (substrate) | **Only in core:** **`templates/core/runtimes/<name>/`** (or bundled **`src/core/runtimes/`**) — substrates are an **engine** concept. | **`runtime`** references an existing profile **name**. Top-level **`runtime`** sets the workflow default; a step may override it. Workflows **do not** define or override substrate types; adding a new runtime is a **core** change. |
| **Resolver** (tool / env profile) | **`templates/core/resolvers/<name>/`** or maintainer trees listed under **`compile.workflows`** (e.g. nested **`…/resolvers/codex/`** with **`profile/`**) | **`resolver`** names the profile. Top-level **`resolver`** sets the workflow default; a step may override it. Package metadata may still declare **`capability`**, but normal workflow authoring should lead with **`resolver`**. |
| **Security policy** | **Core-owned presets** plus engine defaults, compiled into the effective runtime manifest. | Workflow YAML may select `security.profile` and apply bounded `network`, `filesystem`, and `process` overrides. It does **not** expose raw Docker flags or define a second runtime system. |
| **Strategy** (lifecycle wrapper) | **`templates/core/strategies/<name>/`** | **`strategy`**, **`strategies:`** select host before/after scripts. |
| **Domain workflows** (under maintainer packages) | Same as workflows: **`config.yml`** under e.g. **`dockpipe/dorkpipe/`** or nested under **`dockpipe/<group>/resolvers/<name>/`** (resolver-shaped trees include **`profile/`** + workflow assets) | **`scripts/…`** resolves via compiled **`dockpipe-workflow-*`** tarballs first, then source trees. |

So **one repo** can ship **workflows**, **resolvers**, and **runtimes** / **strategies** / the rest of the spine via **`compile core`** ( **`templates/core`** or **`src/core`** ): list **`compile.workflows`** only as the entry point (legacy **`compile.bundles`** merged in; deprecated **`compile.resolvers`** merged if present). Resolver packaging uses the same roots as workflows plus flat **`src/core/resolvers`** and **`templates/core/resolvers`** when those exist. **Runtimes** still live under **core**; workflows only select them. **Compile** emits **`dockpipe-workflow-*`**, **`dockpipe-resolver-*`**, and **`core`** tarballs while **`package.yml`** records **`depends`**, **`namespace`**, and metadata. Step-by-step keys: **[workflow-yaml.md](workflow-yaml.md)**.

### Local compile (`dockpipe build` / `dockpipe package compile` / `dockpipe compile`)

Materialize a **project-local** store under **`bin/.dockpipe/internal/packages/`** without moving authoring trees.

**Repo-root `dockpipe.config.json` (optional)** — JSON with a **`compile`** object:

- **`workflows`** — array of repo-relative (or absolute) **directories** to scan for named workflow folders (each with **`config.yml`**). Same roots drive **`dockpipe package compile resolvers`** (nested **`…/resolvers/<name>/`**) plus flat **`src/core/resolvers`** and **`templates/core/resolvers`** when present.
- **`resolvers`** (deprecated) — optional extra roots merged into resolver compile; prefer listing everything under **`workflows`**.
- **`bundles`** (deprecated) — merged into **`workflows`**; same recursive **`config.yml`** walk.
- **`core_from`** — optional path override for **`compile core`** (same as **`--from`**).
- **`secrets`** (optional) — not secrets themselves; pointers for humans and tooling:
  - **`op_inject_template`** — repo-relative or absolute path to a **mapping file** for **`op inject`** (e.g. **`.env.op.template`** with **`op://`** lines). **`dockpipe doctor`** reports whether that file exists when **`dockpipe.config.json`** is present in the current directory.
  - **`notes`** — free-text reminder (e.g. vault naming, policy).

If **`dockpipe.config.json`** is **missing**, compile uses built-in defaults for each omitted key. **`dockpipe init`** seeds a starter JSON. **Maintainer trees** (e.g. **`.staging/packages`**) are **not** implied: add them explicitly under **`compile.workflows`** when you want them compiled (legacy **`compile.bundles`** is merged into **`compile.workflows`**). See **`.staging/packages/README.md`** in this repo for a typical layout.

Compile steps:

1. **`compile core`** — copies **`src/core`** (default when **`src/core/runtimes`** exists) or **`templates/core`**, or **`compile.core_from`** / **`--from`**, into **`packages/core/`** and writes **`package.yml`** (`kind: core`). **Omits** top-level **`resolvers/`**, **`bundles/`**, and **`workflows/`** from that copy so those slices stay separate packages.
2. **`compile resolvers`** — repeatable **`--from`**; default roots are **`compile.workflows`** (plus legacy **`compile.bundles`** merged in), **`src/core/resolvers`**, and **`templates/core/resolvers`** when those directories exist; deprecated **`compile.resolvers`** entries are merged if present. **Pack roots** under each **`--from`** directory include: **`resolvers/`** (flat); **`packages/<group>/resolvers/`**; **`dockpipe/<group>/resolvers/`** (e.g. dockpipe repo: **`agent`**, **`ide`**, **`secrets`**, **`cloud/storage`**, …). Every immediate child of each pack root that contains **`profile/`** becomes **one** tarball (**`dockpipe-resolver-<name>-…`**) — the **store** still lists **separate** installable resolvers. **`src/core/resolvers`** stays a flat list (no nested **`resolvers/resolvers/`**). **Strategies** and **runtimes** are not packed from the same vendor folder as resolvers; keep lifecycle slices in **`compile core`** until a follow-up convention exists.
3. **`compile workflows`** — every **`config.yml`** under each **`--from`** root (recursive walk). With no config, the default is **`workflows/`** when present. List maintainer roots (e.g. **`.staging/packages`**) in **`compile.workflows`** when you want them compiled; legacy **`compile.bundles`** paths are merged into this list. **`dockpipe package compile bundles`** is an alias for **`compile workflows`**. Optional **`compile_hooks:`** in workflow YAML is a list of **shell** strings run from the workflow source directory **after** validation and **before** the tarball is written (e.g. **`go build`**, codegen). Maintainer layout in this repo: **`.staging/packages/README.md`**.
4. **`compile all`** (alias: **`dockpipe build`**) — runs **core → resolvers → workflows**. **`dockpipe clean`** removes the compiled store; **`dockpipe rebuild`** runs **clean** then **build**.

The runner checks **compiled `packages/resolvers/`** and **`packages/core/`** before **`.staging`** and authoring **`CoreDir`** so you can **opt in** to the compiled store per workdir. Edit **`package.yml`** after compile to add **namespaces**, **`depends`**, and metadata for store-shaped workflows.

## Network boundary: install, not every `run`

**HTTPS / CDN / registry traffic** should be confined to **explicit install (and publish)** commands — e.g. **`dockpipe install core`**, future **`dockpipe install package …`**, **`dockpipe release upload`**. After artifacts are on disk, **`dockpipe run`** against local workflows or installed packages should **not** need network unless the **workflow itself** does (e.g. `docker pull`, API calls).

Package metadata may declare an OCI image reference as a **hint/reference**:

```yaml
image:
  source: registry
  ref: ghcr.io/acme/tool@sha256:...
  pull_policy: if-missing
```

The package manifest does **not** become runtime truth. Compile folds the reference into the effective runtime/image manifests, and run consumes those compiled manifests. If the image is already local and valid, run stays local. If it is missing, run may pull only when the compiled pull policy and compiled network policy allow it.

## Lifecycle: compile → package → release

1. **`compile`** — Validate workflow YAML and **materialize** a **self-contained** tree: copy the workflow and (as the implementation grows) **pull in domain-specific assets** referenced from source so the compiled directory is the **single** execution root for that package.
2. **`package`** — Archive **that compiled tree** (plus **`package.yml`**, checksums, optional lock metadata). **`dockpipe package build store`** turns the compiled store into **gzip tarballs** (one per core / workflow / resolver) and **`packages-store-manifest.json`**. **`dockpipe run --workflow`** can **stream** a workflow from **`dockpipe-workflow-<name>-*.tar.gz`** when **`config.yml` in the archive** sets **`namespace:`** and on-disk paths do not win (see **`packages.tarball_dir`** / **`packages.namespace`** in **`dockpipe.config.json`**). **`dockpipe package build core`** builds the **`templates/core`** artifact for **`dockpipe install core`**.
3. **`release`** — Upload tarball + manifest to your **static origin**; consumers **`install`** to pull it.

**CI** can chain the same steps locally: **compile → package → release**.

## Workflow install and resolver dependencies

When **`dockpipe install`** (workflow package) exists end-to-end, installing a **workflow** should **also install declared dependencies**, primarily **`kind: resolver`** packages (**`depends`** / pins in **`package.yml`**), including a **transitive** closure where needed. **Domain-specific** scripts and assets belong **inside** the workflow package / compile output — not as a separate CDN hop for every file at run time. **Resolver** packages remain **shared adapters** (tool profiles, resolver-owned assets).

Package dependencies should remain package-shaped:

- **`depends`** names other package ids.
- **`requires_resolvers`** names resolver profile ids that must be available project-locally, globally, or from the install closure.
- **`requires_capabilities`** names dotted capability ids for catalog/search and dependency checks.
- **`image`** may point at a normal OCI reference, ideally digest-pinned, but Docker layers remain in Docker/OCI registries rather than DockPipe package tarballs.

Security metadata in **`package.yml`** should stay compatibility-only, for example **`compatible_security_profiles`** or **`requires_network: true`** if added later. Effective network, filesystem, process, and Docker enforcement settings belong only in compiled runtime manifests.

## Distribution split (repo vs store)

| What | Where it usually lives | Notes |
|------|-------------------------|--------|
| **Runtimes** | **Repo / bundled `templates/core/runtimes/`** | Stable, light profiles — **not** the main “store” surface. |
| **Strategies** | **Repo / bundled `templates/core/strategies/`** | Thin lifecycle wiring — same as runtimes: **keep in tree**. |
| **Compiled core** | **HTTPS/S3 (e.g. R2)** + **`dockpipe install core`** | **Tight `templates/core` tarball** so installs stay small; refresh without cloning the whole upstream repo. |
| **Resolvers** | **Bundled** and/or **store packages** | **Plugin adapters** — shared across workflows; extended catalogs ship as packages with rich **`package.yml`**. |
| **Workflows** | **Authoring tree**, **`bin/.dockpipe/internal/packages/workflows/`**, or **store** | **Primary rich-metadata** packages for authoring and discovery (`kind: workflow`). |

**Mental model:** the **CLI + slim core** in git or from S3 gives you a **lightweight spine**; **workflows** and **resolver** packs are what you **browse, version, and install** from a registry or internal bucket (the “plugin store” layer).

## 1. Packages (installed, store-backed)

**Packages** are **self-contained** artifacts you fetch from an object store (e.g. **Cloudflare R2** behind HTTPS) or another registry. They are **building blocks** for YAML workflows: full workflows, slices of **`templates/core`** (resolvers, runtimes, strategies, assets), or extra workflow tarballs from package installs.

- **Default layout on disk:** **`<workdir>/bin/.dockpipe/internal/packages/`** — kept under **`internal/`** so user-created and installable packages stay separate from other **`bin/.dockpipe/`** state (runs, handoffs, CI).  
  Override with **`DOCKPIPE_PACKAGES_ROOT`** (absolute path, or relative to workdir), e.g. **`vendor/dockpipe-packages`** if you want packages **versioned in git** without fighting a blanket **`.dockpipe/`** ignore.

Suggested subdirectories (mirror authoring concepts; not all are required):

| Path | Role |
|------|------|
| **`bin/.dockpipe/internal/packages/workflows/<name>/`** | Workflow-shaped trees (`config.yml`, steps, …). |
| **`bin/.dockpipe/internal/packages/core/`** | Compiled **spine** only: **`runtimes/`**, **`strategies/`**, **`assets/`**, etc. — not resolver/bundle/workflow packages. |
| **`bin/.dockpipe/internal/packages/resolvers/<name>/`** | One resolver package per profile (same shape as **`templates/core/resolvers/<name>/`**). |
| **`bin/.dockpipe/internal/packages/workflows/`** | Workflow packages (**`dockpipe-workflow-*`**). |
| **`bin/.dockpipe/internal/packages/assets/`** | Optional top-level packs (e.g. large binaries) that are not folded under **`core/`**. |

**Metadata:** each installable unit should include **`package.yml`** next to its payload (see **`dockpipe package manifest`**). Core fields: **`name`**, **`version`**, **`title`**, **`description`**, **`author`**, **`website`**, **`license`**, **`kind`** (`workflow` \| `resolver` \| `core` \| `assets` \| `bundle`). Optional **`namespace`** — same rules as workflow **`config.yml`** **`namespace:`** (lowercase label; reserved words like **`dockpipe`**, **`core`**, **`system`** are rejected).

**Versioning:** treat **`package.yml version`** as the release identity for that package. In this repo, authored package metadata under **`packages/`**, **`.staging/packages/`**, and bundled example package metadata under **`src/core/`** should normally track the repo-root **`VERSION`** unless a package is intentionally released on a different cadence. Generated manifests from **`dockpipe package compile`** inherit repo-root **`VERSION`** by default when present; explicit versions must be semver-shaped so tarball names and CDN paths stay stable.

**Rich metadata (authoring & store discovery)** — optional but recommended for **workflow** and **resolver** packages:

| Field | Purpose |
|-------|---------|
| **`provider`** | Optional platform or vendor id for filtering and catalog facets (e.g. **`cloudflare`**, **`aws`**, **`github`**) — short stable label, not a URL |
| **`capability`** | For **`kind: resolver`** — dotted id this package provides (e.g. **`cli.codex`**, **`blob.storage`**) — see **[capabilities.md](capabilities.md)** |
| **`requires_capabilities`** | For **`kind: workflow`** — dotted capability ids this workflow expects (complements **`requires_resolvers`**) |
| **`tags`**, **`keywords`** | Faceted search / UI filters |
| **`min_dockpipe_version`** | Semver constraint on the CLI |
| **`repository`** | Source repo URL |
| **`provides`** | Resolver capability names (e.g. tool ids) for **`kind: resolver`** |
| **`requires_resolvers`** | Hint compatible resolver profiles for **`kind: workflow`** |
| **`depends`** | Other package **names** this package expects |
| **`namespace`** | Author/org label for discovery and future namespaced installs (validated; see **`domain.ValidateNamespace`**) |
| **`allow_clone`** | If **`true`**, **`dockpipe clone`** may export the compiled tree to **`workflows/`**; if false or omitted, clone is refused. |
| **`distribution`** | Optional hint: **`source`** or **`binary`** (documentation for store pages). |
| **`image`** | Optional normal OCI image reference for a workflow package. Compile records it into the effective runtime/image manifests; `run` may reuse or pull it according to compiled pull policy. |

The Go type **`domain.PackageManifest`** parses these keys; see **`src/lib/domain/package_manifest.go`**.

**Compression:** store objects are typically **`.tar.gz`** (or **`.tar.zst`** later) to keep bandwidth and R2 storage small; the CLI unpacks into the layout above. **Binary-only** packs are possible for asset-only packages if you add a small unpack step later.

**CLI today:** **`dockpipe install core`** pulls **`templates/core`** into **`templates/core`** (bootstrap path). **`dockpipe package compile workflow`** validates YAML and copies a workflow tree into **`bin/.dockpipe/internal/packages/workflows/<name>/`** (see **`docs/cli-reference.md`**). Future **`dockpipe install package …`** will unpack registry artifacts and extend resolution order (project **`workflows/`** + **installed packages** + embedded bundle).

### Clone, education, and commercial packages

- **`package.yml`** may set **`allow_clone: true`** so **`dockpipe clone <name>`** can copy a compiled workflow package from **`bin/.dockpipe/internal/packages/workflows/<name>/`** into **`workflows/<name>/`** (or **`--to`**). This supports **education** and **forking** when the author opts in.
- **`allow_clone: false`** or omitting **`allow_clone`** means **`dockpipe clone`** refuses — appropriate for **commercial** or **binary-only** releases where the publisher does not grant a recoverable source tree.
- Optional **`distribution: binary`** documents that the published artifact is not meant for source recovery; publishers who need **strong** protection should ship **only** non-recoverable binaries (obfuscation, native modules, etc.) — DockPipe cannot cryptographically enforce that; **`allow_clone`** is the explicit license bit for **`dockpipe clone`**.
- **`dockpipe package compile workflow`** writes **`allow_clone: true`** and **`distribution: source`** into a generated **`package.yml`** so local compiles stay cloneable unless you edit the manifest before release.

## 2. Uncompressed working tree (authoring / clone)

**Not** “packages” in this sense: the repo you edit every day.

- **`workflows/`** (default project root), **`src/core/workflows/<name>/`** (bundled examples in the dockpipe repo), legacy **`templates/<name>/`** — YAML and assets you **build or clone**, commit, and run with **`--workflow`** as today.
- No **`package.yml`** required; this is normal development.

## 3. Project-local state (`bin/.dockpipe/`) and isolation

**`bin/.dockpipe/`** is the **project-local** tree for generated state: host run records (**`bin/.dockpipe/runs/`**), step outputs (**`bin/.dockpipe/outputs.env`**), handoffs, optional demo stubs, and **installed package material** under **`bin/.dockpipe/internal/`**. That keeps **transient and tool-owned** files out of the main authoring trees and the repo root.

**`bin/.dockpipe/internal/packages/`** is the default store for **fetched or compiled** package trees (workflows, core slices, assets) — the same conceptual layout whether content arrived as a **`.tar.gz`** or from **`dockpipe package compile workflow`**. **Uncompressed** authoring under **`workflows/`** remains normal; **compile** validates and **materializes** into **`bin/.dockpipe/internal/...`** when you opt into the packaged path. Resolution order for **`--workflow`** is implemented in **`workflow_dirs.go`** (**`workflows/`** → **packages** → legacy **`templates/`** paths, etc.).

Image artifact metadata is **not** a package payload by default. Use:

- **`bin/.dockpipe/internal/images/`** for project-local image artifact indexes.
- **`bin/.dockpipe/internal/cache/images/`** for project-local cached image artifact records.
- **`<global-root>/images/`** for global image artifact indexes.

Compiled workflow tarballs may contain **`.dockpipe/image-artifact.json`** and per-step image manifests as inspectable compiled truth, but the local/global image index is separate from **`packages/`** so packages, runtime manifests, image artifacts, and run records do not blur together.

## 4. Global installs

Global installs are for user-wide DockPipe extensions. They do not live under **`bin/.dockpipe`** because there may be no project checkout involved.

Suggested global layout:

| Path | Role |
|------|------|
| **`<global-root>/templates/core/`** | Globally installed core spine from **`dockpipe install core --global`**. |
| **`<global-root>/packages/workflows/`** | Globally installed workflow packages. |
| **`<global-root>/packages/resolvers/`** | Globally installed resolver packages. |
| **`<global-root>/packages/assets/`** | Globally installed shared asset packages. |
| **`<global-root>/images/`** | Global image artifact metadata/indexes. |
| **`<global-root>/cache/`** | Global download/cache metadata. |

Project-local packages should win over global packages so a repository can pin or override its own dependency closure. Global packages are the shared fallback.

**Publish outputs** (templates-core tarball, checksums, GitHub release binaries in CI) live under **`release/artifacts/`** (gitignored), not the project workflow tree — see **`release/README.md`**.

**Direction:** stronger **validation** (schema, lint) at **compile** time; optional **package** / **install** for store-backed workflows; **source** mode stays available for low-friction authoring.

## Resolution order (directional)

**`--workflow`** resolution (see **`workflow_dirs.go`**) already checks **`bin/.dockpipe/internal/packages/workflows/<name>/`** (after **`workflows/`** and before legacy **`templates/<name>/`**) when **`dockpipe run`** uses **`--workdir`** or the current directory; **`dockpipe doctor`** and **`ResolveWorkflowConfigPath(repoRoot, name)`** without a workdir skip the packages store.

When fully wired end-to-end, workflow name resolution will **prefer** project **`workflows/`**, then project-local **`bin/.dockpipe/internal/packages/workflows/`**, then global **`<global-root>/packages/workflows/`**, then legacy **`templates/`** paths and the embedded bundle — same four concepts (**workflow**, **runtime**, **resolver**, **strategy**), extended by **packages** from the store.

See also **[architecture-model.md](architecture-model.md)** and **[cli-reference.md](cli-reference.md)** (`dockpipe package`, `dockpipe install`). For **core vs optional packages** and an untethering roadmap (slim core, explicit `depends`), see **[core-vs-packages-audit.md](core-vs-packages-audit.md)**.
