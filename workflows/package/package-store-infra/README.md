# package-store-infra

**Thin composer** for this repo: **`vars`** (canonical non-secret **`TF_VAR_*`**, **`DOCKPIPE_TF_*`**, **`R2_*`**) plus a nested packaged workflow step:

```yaml
steps:
  - workflow: dockpipe.cloudflare.r2infra
    package: dockpipe-cloudflare
```

Same packaged workflow as **`dockpipe --workflow dockpipe.cloudflare.r2infra`** alone; this file is where you **centralize env** and **add more steps later** (e.g. another nested package, init/apply ordering) **without** re‑implementing what the package already does.

**Does not** run **`dockpipe package build store`** — that’s separate when you need **`release/artifacts`** tarballs + **`packages-store-manifest.json`**. After compile: **`dockpipe package build store`**, then upload flows (`r2upload`, etc.).

## Run

```bash
./src/bin/dockpipe --workflow package-store-infra --
```

**Terraform:** **`--tf plan`** or **`--tf apply`**. With **`DOCKPIPE_TF_OPTIONAL_WHEN_UNSET=1`** (default here), runs without **`--tf`** skip Terraform inside **`r2-publish.sh`**.

**Compile:** Default pre-run is **compile for-workflow** for this name. **`dockpipe build`** (compile all) then **`--no-compile-deps`** if you already compiled fully.
