# Terraform pipeline helper (`terraform-pipeline.sh`)

Host workflows can run **Terraform** as plain **`cli`** steps (see [architecture-model.md](architecture-model.md): Terraform is **not** a DockPipe “runtime”; it is a tool invoked from a script).

This repo ships a **reusable bash library**:

- **`src/core/assets/scripts/terraform-pipeline.sh`**
- After **`dockpipe init`**, the same path appears under **`templates/core/assets/scripts/terraform-pipeline.sh`**.

## From the `dockpipe` binary

The same file workflow YAML names as **`scripts/core.assets.scripts.terraform-pipeline.sh`** can be resolved without guessing paths:

```bash
dockpipe terraform pipeline-path
# or, for any core namespaced asset:
dockpipe core script-path assets.scripts.terraform-pipeline.sh
```

That uses the same resolution rules as workflow **`scripts/`** paths (bundled core, `templates/core`, overlays, etc.). Set **`DOCKPIPE_REPO_ROOT`** when developing against a full git checkout.

## Source it from your workflow script

```bash
ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
for candidate in "$ROOT/templates/core/assets/scripts/terraform-pipeline.sh" "$ROOT/src/core/assets/scripts/terraform-pipeline.sh"; do
  if [[ -f "$candidate" ]]; then
    # shellcheck source=/dev/null
    source "$candidate"
    break
  fi
done
export DOCKPIPE_TF_LOG_PREFIX=my-workflow
```

Set **`TF_VAR_*`** (and any other env Terraform should see), then call:

```bash
dockpipe_tf_run_pipeline "/path/to/terraform/module" "/tmp/backend.hcl" "optional dry-run hint string"
```

The second argument is only used when **`DOCKPIPE_TF_BACKEND=remote`** (R2 S3-compatible state backend); for local state use **`DOCKPIPE_TF_BACKEND=local`** and pass a dummy path or extend the helper for your backend.

## Environment: `DOCKPIPE_TF_*` (convention)

| Variable | Meaning |
|----------|---------|
| `DOCKPIPE_TF_COMMANDS` | Comma-separated steps: **`init`**, **`plan`**, **`apply`**, **`validate`**, **`fmt`**, **`import`**. Default in examples: `init,apply`. |
| `DOCKPIPE_TF_SKIP_INIT` | `1` to skip auto-prepending **`init`**. |
| `DOCKPIPE_TF_BACKEND` | `local` or **`remote`** (R2 state file via S3 backend config). |
| `DOCKPIPE_TF_STATE_BUCKET` / `DOCKPIPE_TF_STATE_KEY` | Remote state object (R2). |
| `DOCKPIPE_TF_STATE_ACCESS_KEY_ID` / `DOCKPIPE_TF_STATE_SECRET_ACCESS_KEY` | Credentials for state bucket (or fall through to **`R2_STATE_*`** / **`AWS_*`** in the **`dockpipe_tf_map_r2_publish_env`** mapper). |
| `DOCKPIPE_TF_CLOUDFLARE_ACCOUNT_ID` | For writing the R2 endpoint in backend HCL (defaults to **`CLOUDFLARE_ACCOUNT_ID`**). |
| `DOCKPIPE_TF_WORKSPACE` | If set, after **`init`** the helper runs **`terraform workspace select`** or **`new`**. |
| `DOCKPIPE_TF_INIT_ARGS` / `PLAN_ARGS` / `APPLY_ARGS` / `VALIDATE_ARGS` / `FMT_ARGS` | Extra arguments (space-separated) passed to the matching subcommand. |
| `DOCKPIPE_TF_APPLY_AUTO_APPROVE` | `1` (default) adds **`-auto-approve`** to **`apply`**. |
| `DOCKPIPE_TF_IMPORT_ARGS` | One **`terraform import`** invocation: space-separated tokens after **`terraform import -input=false`**. |
| `DOCKPIPE_TF_IMPORT_FILE` | Path to a file: **one import per line**, **`ADDRESS`** then rest of line is **`ID`** (supports spaces in ID). Lines starting with **`#`** are ignored. |
| `DOCKPIPE_TF_DRY_RUN` | `1` prints the pipeline without running Terraform. |

**`import` step:** include **`import`** in **`DOCKPIPE_TF_COMMANDS`** and set **`DOCKPIPE_TF_IMPORT_ARGS`** and/or **`DOCKPIPE_TF_IMPORT_FILE`**. Run **`init`** before **`import`** (auto-prepended when needed).

## `dockpipe.cloudflare.r2publish` compatibility

The **`dockpipe.cloudflare.r2publish`** workflow (host script **`scripts/dockpipe/r2-publish.sh`**) maps legacy **`R2_TERRAFORM_*`**, **`R2_TF_*`**, and **`R2_PUBLISH_DRY_RUN`** into **`DOCKPIPE_TF_*`** via **`dockpipe_tf_map_r2_publish_env`**. You can set either naming; **`DOCKPIPE_TF_*`** wins when both are set.

## See also

- [templates-core-assets.md](templates-core-assets.md) — where **`assets/scripts`** lives in the bundle.
- **`.staging/packages/dockpipe/cloud/storage/resolvers/r2/dockpipe.cloudflare.r2publish/README.md`** — end-to-end R2 upload + Terraform.
