# Core vs packages — audit and untethering roadmap

This doc aligns with the **compile → package → optional install** direction: keep the **engine** and the **smallest useful core slice** in the binary, and treat everything else as **named packages with `depends`**.

## Two different “core” surfaces (do not conflate)

| Surface | What it is | What it contains today |
|--------|------------|-------------------------|
| **Embedded FS** (`embed.go`) | Files baked into the `dockpipe` binary | `src/core/**`, `workflows/**`, `packages/**`, `.staging/packages`, etc. — **full trees** |
| **`dockpipe package compile core`** | Tarball under **`.dockpipe/internal/packages/core/`** | Copy of **`src/core`** (or `templates/core`) **excluding** top-level **`resolvers/`**, **`bundles/`**, **`workflows/`** — see `copyDirExcludingTopLevel` in **`src/lib/application/package_compile.go`** |

So: **example workflows** and **`src/core/resolvers/*`** are **not** in the **compiled core tarball**, but they **are** still in the **embedded checkout** for a full clone. Untethering is partly about **embed** and partly about **where** scripts live on disk.

## What should stay “engine” (not optional)

- **`src/lib/`**, **`src/cmd/`** — workflow execution, isolation, resolution, flags, compile pipeline.
- **Runtimes / strategies** under **`src/core/runtimes`**, **`src/core/strategies`** — substrate definitions referenced by YAML.
- **Minimal shared assets** truly required for **`dockpipe init`** and default **`--workflow`** resolution (thin, not vendor-specific).

## Candidates to untether (lighter core, explicit dependencies)

### 1. Terraform helpers (high value) — **done (source tree)**

| Today | Location | Target |
|-------|----------|--------|
| `terraform-pipeline.sh` | ~~`src/core/assets/scripts/`~~ | **`packages/terraform/resolvers/terraform-core/assets/scripts/`** — **`scripts/core.assets.scripts.*`** resolves here **first** (`terraformPackCoreAssetPath` in **`paths.go`**) |
| `terraform-run.sh` | ~~`src/core/assets/scripts/`~~ | Same |
| `dockpipe terraform` subcommand | CLI | **`pipeline-path`** uses same resolution as workflow YAML |
| `--tf` flags | `run` | Unchanged semantics; env still **`DOCKPIPE_TF_*`** |

**Note:** Compiled **`templates/core`** tarballs from **`package build core`** no longer carry these scripts; projects that only install core and need Terraform should keep **`packages/terraform`** in the tree or vendor the pack.

**Embedded binary:** `scripts/core.assets.scripts.terraform-*` resolves to **`bundle/workflows/terraform-core/assets/scripts/`** under the materialized bundle cache (`~/.cache/dockpipe/bundled-<ver>/`, same `terraform-core` tree as **`packages/terraform/resolvers/terraform-core`** in a git checkout). **`bundledFormatVersion`** bumps force re-unpack when layout changes.

### 2. Cloudflare / R2 (already partly separated)

| Today | Location |
|-------|----------|
| `terraform-cloudflare-r2-run.sh` | `packages/cloud/storage/resolvers/r2/dockpipe.cloudflare.r2publish/assets/scripts/` |
| `r2-publish.sh` | `packages/dorkpipe/.../scripts/` |

**Target:** `package.yml` **`depends`** / **`requires_resolvers`** for workflows that nest R2; no Cloudflare imports in **`src/lib`**.

### 3. Heavy or demo Dockerfiles / compose

| Area | Untether idea |
|------|----------------|
| `src/core/assets/images/dev`, `example`, `base-dev` | Optional **`kind: assets`** or **`dockpipe-core-images`** pack (or lazy pull) |
| `src/core/assets/compose/*` | Same — only **minimal** compose stays if the default workflow needs it |

### 4. Bundled example workflows (`src/core/workflows/*`)

| Today | Still embedded for **`dockpipe init --from`** |
|-------|-----------------------------------------------|
| `run`, `init`, `run-apply`, … | Could ship **only** via **HTTP install** / **registry** once **`dockpipe install`** is the default path |

**Effort:** high — product decision; **embed** shrink is a release-size win.

### 5. `src/core/resolvers/example`, `dotenv`

| Today | Excluded from **compile core** tree but present in **embed** |
|-------|---------------------------------------------------------------|
| Purpose | Examples / small profiles |

**Target:** move under **`packages/`** only; **`src/core/resolvers`** reserved for **true** bundled defaults only, or empty.

### 6. DorkPipe maintainer scripts (`packages/dorkpipe/...`)

Already **not** `src/core`; keep as **maintainer** package.

## Manifest `depends` (compile core)

Generated **`dockpipe-core-*.tar.gz`** currently has **`depends: []`** in `package_compile.go`. A future step is to **emit** optional **`depends`** (e.g. `terraform` workflow pack) when the core slice no longer ships those scripts.

## Suggested phases

1. **Document** — this file; keep **AGENTS.md** / **package-model.md** aligned.
2. **Move Terraform scripts** from `src/core/assets/scripts/` into **`packages/terraform`** assets; resolve **workflow YAML** and **`dockpipe terraform`** from that package first.
3. **Shrink embed** (optional build tag or split artifact) — ship **slim** binary + **`dockpipe install`** packs for examples and Terraform.
4. **Resolver examples** — relocate **`src/core/resolvers/example`** / **`dotenv`** to **`packages/`** and update **`init`** templates.

## References

- **`docs/package-model.md`** — compile order, install semantics
- **`embed.go`** — what the binary embeds
- **`package_compile.go`** — `cmdPackageCompileCore` exclude list
