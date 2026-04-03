# `dockpipe.terraform.core`

**Provider-agnostic** host workflow: **`assets/scripts/terraform-pipeline.sh`** + **`assets/scripts/terraform-run.sh`** in this resolver tree. In YAML use **`scripts/core.assets.scripts.terraform-run.sh`** (resolution maps to this package).

## Required

- **`DOCKPIPE_TF_MODULE_DIR`** — path to the Terraform root directory (`*.tf`), repo-relative to `--workdir` or absolute.

## Remote state

Use **`DOCKPIPE_TF_REMOTE_BACKEND_FILE`** with a backend HCL you maintain (any backend your Terraform version supports), or configure the **`backend`** block in your module and use **`DOCKPIPE_TF_BACKEND=local`** with appropriate provider env vars.

**Do not** put Cloudflare-, R2-, or AWS-specific env keys in this workflow — those belong in **`dockpipe.cloudflare.r2infra`**, **`dockpipe.cloudflare.r2publish`**, and related **`packages/cloud/storage/...`** workflows.

## Common variables

| Variable | Purpose |
|----------|---------|
| `DOCKPIPE_TF_COMMANDS` | Comma-separated: `init`, `plan`, `apply`, … (default `plan` via generic map). |
| `DOCKPIPE_TF_BACKEND` | `local` or `remote` (remote + portable backends: use `DOCKPIPE_TF_REMOTE_BACKEND_FILE`). |
| `DOCKPIPE_TF_REMOTE_BACKEND_FILE` | Path to backend HCL for `terraform init -backend-config=`. |
| Other `DOCKPIPE_TF_*` | See **`src/core/assets/scripts/README.md`** (terraform env table; library lives in this package). |

## Umbrella package

Declared in **`packages/terraform/package.yml`** (`includes_resolvers: terraform-core`).
