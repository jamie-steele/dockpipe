# Cloudflare R2 — Terraform module + `r2-publish.sh`

This folder keeps the **Terraform module** under **`terraform/`** (paths and state keys still use the historical name **`dockpipe.cloudflare.r2publish`**). The **Cloudflare/R2 Terraform host script** (thin wrapper around **`terraform-core`**’s **`terraform-pipeline.sh`**) lives with the infra workflow resolver: **`packages/cloud/storage/resolvers/r2/dockpipe.cloudflare.r2infra/assets/scripts/terraform-cloudflare-r2-run.sh`**, referenced in YAML as **`scripts/dockpipe.cloudflare.r2infra/terraform-cloudflare-r2-run.sh`**. **Tar + upload:** **`scripts/dockpipe/r2-publish.sh`**. Provider-agnostic Terraform is **`packages/terraform/resolvers/terraform-core/assets/scripts/terraform-run.sh`** (**`dockpipe.terraform.core`** only; **`scripts/core.assets.scripts.terraform-run.sh`** resolves there).

**Infra and object upload are separate workflows** (same script, different env):

| Workflow | What runs |
|----------|-------------|
| **`dockpipe.cloudflare.r2infra`** | Terraform only via **`scripts/dockpipe.cloudflare.r2infra/terraform-cloudflare-r2-run.sh`** (Cloudflare/R2 host; **`r2-publish.sh`** resolves the same file). No tarball, no upload; does not require **`release/artifacts`**. |
| **`dockpipe.cloudflare.r2upload`** | Tar **`R2_PUBLISH_SOURCE`** and upload — **`R2_SKIP_TERRAFORM=1`**. Run after **`r2infra`** (or when the bucket already exists). |

Typical order: **`r2infra`** (or **`package-store-infra`** with nested r2infra + shared vars) → **`dockpipe package build store`** when you need store tarballs → **`r2upload`**.

### Design mock (pipeline shape)

**`config.design-mock.yml`** shows **multi-step** packaging (`runtime: package` + nested workflow). See **`docs/architecture-model.md`**.

## Two ways to authenticate

| Mode | Credentials | Upload | Bucket |
|------|-------------|--------|--------|
| **S3 API** | `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` (R2 access keys) | `aws s3 cp` | You create the bucket (or use Terraform separately). |
| **Single API token** | `CLOUDFLARE_API_TOKEN` + `CLOUDFLARE_ACCOUNT_ID` | Terraform **creates** the bucket, then **`wrangler r2 object put`** | Created by Terraform unless you skip it. |

**Terraform state** defaults to **remote** on R2: bucket **`dockpipe`**, object key **`state/dockpipe.cloudflare.r2publish/terraform.tfstate`** (same account as `CLOUDFLARE_ACCOUNT_ID`). If you used **`state/r2-publish/terraform.tfstate`**, set **`R2_TF_STATE_KEY`** until you migrate. The S3-compatible backend uses **R2 access keys** (`R2_STATE_*` or `AWS_*`), not the Cloudflare API token — see [Remote R2 backend](https://developers.cloudflare.com/terraform/advanced-topics/remote-backend/). If the publish bucket already exists, **`terraform import`** before apply — see **`terraform/README.md`**.

R2 is the **destination**, not a DockPipe runtime. **`cloudflare`** is not a runtime; the runtime here is **`cli`** on the host (`skip_container: true`).

**Consumers:** After you publish **`release/artifacts/`** (or a custom prefix), serve objects at a **public HTTPS hostname**. The Terraform module under **`terraform/`** can create an **R2 custom domain**, **WAF** (managed rules), and **cache rules** (CDN) when you set the feature flags — see **`terraform/README.md`**. Alternatively, attach a hostname in the Cloudflare dashboard or via a Worker. Downstream projects can run **`dockpipe install core --base-url https://your-cdn.example/dockpipe`** (or **`DOCKPIPE_INSTALL_BASE_URL`**) to replace **`templates/core/`** without cloning the full dockpipe repo. Build artifacts with **`make package-templates-core`** or **`dockpipe package build core`** at the repo root.

## Prerequisites

- **`release/artifacts/`** (or `R2_PUBLISH_SOURCE`) with something to pack
- **`R2_BUCKET`** — bucket name (Terraform uses this name when creating the bucket)

### S3 mode

- **aws** CLI v2
- **Endpoint** — `R2_ENDPOINT_URL=https://<account_id>.r2.cloudflarestorage.com` or **`CLOUDFLARE_ACCOUNT_ID`** so the script can build the URL

See [Cloudflare R2 — AWS CLI](https://developers.cloudflare.com/r2/examples/aws/aws-cli/).

### Single-token mode (Terraform + Wrangler)

- **Terraform** (`terraform` on `PATH`) — module at **`…/dockpipe.cloudflare.r2publish/terraform`** (or `R2_TERRAFORM_DIR`); use workflow **`dockpipe.cloudflare.r2infra`**
- **Wrangler** — either `wrangler` on `PATH`, or **Node.js** (`npx` runs `wrangler@3`)
- **API token** with at least **Account → R2 → Edit** (bucket + objects). If Terraform also creates a **custom domain**, **WAF**, or **cache rules**, the token needs **zone** permissions for that hostname’s zone — see **`terraform/README.md`**. See [R2 API tokens](https://developers.cloudflare.com/r2/api/tokens/).
- **R2 API keys for Terraform state** — scoped to the **`dockpipe`** bucket (Object Read & Write). Set **`R2_STATE_ACCESS_KEY_ID`** / **`R2_STATE_SECRET_ACCESS_KEY`**, or use the same **`AWS_*`** pair if it already targets that bucket. Use **`R2_TF_BACKEND=local`** to keep state only on disk (no keys).

Terraform module: [Cloudflare provider + R2](https://developers.cloudflare.com/r2/examples/terraform/) (`cloudflare_r2_bucket`, optional `cloudflare_r2_custom_domain` and rulesets; provider **`~> 5.0`**, lock file: `terraform/.terraform.lock.hcl`). **Import** existing buckets: **`terraform/README.md`**.

## Run (this repo)

### 1) Terraform only (API token mode)

```bash
export R2_BUCKET=your-bucket
export CLOUDFLARE_ACCOUNT_ID=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
export CLOUDFLARE_API_TOKEN=...
dockpipe --workflow dockpipe.cloudflare.r2infra
```

### 2) Upload tarball only (S3 keys — bucket must exist)

```bash
make build   # if needed
export R2_BUCKET=your-bucket
export CLOUDFLARE_ACCOUNT_ID=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
dockpipe --workflow dockpipe.cloudflare.r2upload
```

### 2) Upload only (API token + Wrangler)

```bash
export R2_BUCKET=your-bucket
export CLOUDFLARE_ACCOUNT_ID=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
export CLOUDFLARE_API_TOKEN=...
dockpipe --workflow dockpipe.cloudflare.r2upload
```

Dry run upload (no object put):

```bash
R2_PUBLISH_DRY_RUN=1 dockpipe --workflow dockpipe.cloudflare.r2upload
```

## Environment variables

| Variable | Default | Meaning |
|----------|---------|---------|
| `R2_PUBLISH_SOURCE` | `release/artifacts` | Directory to tar (relative to workdir). |
| `R2_PREFIX` | *(empty)* | Key prefix in the bucket, e.g. `releases/` |
| `R2_ARCHIVE_NAME` | `dockpipe-publish-YYYYMMDD-HHMMSS.tar.gz` | Object name in the bucket. |
| `R2_ENDPOINT_URL` | *(from account id, S3 mode)* | Override S3 endpoint URL. |
| `R2_CONTENT_TYPE` | `application/gzip` | Wrangler object `Content-Type` (token mode). |
| `R2_TERRAFORM_DIR` | *(auto)* | Path to Terraform module (default: `workflows/dockpipe.cloudflare.r2publish/terraform` or `templates/dockpipe.cloudflare.r2publish/terraform` under workdir; legacy `templates/r2-publish/terraform` still searched). |
| `R2_TF_LOCATION` | *(omit)* | Optional `location` for `cloudflare_r2_bucket` (e.g. `WEUR`). |
| `R2_SKIP_TERRAFORM` | `0` | Set to `1` to skip Terraform ( **`dockpipe.cloudflare.r2upload`** sets this). |
| `R2_USE_TERRAFORM` | *(see below)* | `1` to always run Terraform; `0` to never run. |
| `R2_PUBLISH_DRY_RUN` | `0` | Set to `1` to print only what would run. |
| `R2_TF_BACKEND` | `remote` | `remote` — state in R2 (`dockpipe` / `state/dockpipe.cloudflare.r2publish/terraform.tfstate` by default). `local` — `terraform init -backend=false` (no R2 keys). |
| `R2_TF_STATE_BUCKET` | `dockpipe` | State object bucket (S3 backend). |
| `R2_TF_STATE_KEY` | `state/dockpipe.cloudflare.r2publish/terraform.tfstate` | State object key inside that bucket. |
| `R2_STATE_ACCESS_KEY_ID` | *(unset)* | R2 S3 API access key for the **state** bucket (or use `AWS_ACCESS_KEY_ID` when it targets that bucket). |
| `R2_STATE_SECRET_ACCESS_KEY` | *(unset)* | Secret for `R2_STATE_ACCESS_KEY_ID`. |

### Terraform: `init` / `plan` / `apply` (same as any env — `--env`, workflow `.env`, `export`)

**CLI (preferred):** pass Terraform steps on the same invocation as your workflow:

```bash
# From the repo root (usual case): --workdir is optional — default is the current directory.
dockpipe --workflow dockpipe.cloudflare.r2infra --tf plan
dockpipe --workflow dockpipe.cloudflare.r2infra --tf apply
```

Use **`--workdir /path/to/project`** when the shell is not already in that directory (CI, scripts, **`make`**). Also **`--tf-dry-run`**, **`--tf-no-auto-approve`**, and **`--tf=plan`**. Standalone helper: **`dockpipe terraform plan`** (see **`dockpipe terraform --help`**). Workflows that never source **`terraform-pipeline.sh`** ignore these env vars.

Implementation is the shared library **`packages/terraform/resolvers/terraform-core/assets/scripts/terraform-pipeline.sh`** (or **`templates/core/...`** after a copy), mapped from the **`R2_*`** names below. The canonical env namespace is **`DOCKPIPE_TF_*`** — full list in **`src/core/assets/scripts/README.md`** (terraform section) and **`packages/terraform/resolvers/terraform-core/README.md`** (including **`validate`**, **`fmt`**, **`import`**, **`DOCKPIPE_TF_WORKSPACE`**, and import file format).

| Variable | Default | Meaning |
|----------|---------|---------|
| `R2_TERRAFORM_COMMANDS` | `init,apply` | Comma-separated steps: `init`, `plan`, `apply`, `validate`, `fmt`, `import` (order kept). `init` is auto-prepended when needed (unless `R2_TERRAFORM_SKIP_INIT=1`). |
| `R2_TERRAFORM_SKIP_INIT` | `0` | Set to `1` to skip `init` (only if state already initialized). |
| `R2_TERRAFORM_INIT_ARGS` | *(empty)* | Extra args for `terraform init` (space-separated). |
| `R2_TERRAFORM_PLAN_ARGS` | *(empty)* | Extra args for `terraform plan`. |
| `R2_TERRAFORM_APPLY_ARGS` | *(empty)* | Extra args for `terraform apply`. |
| `R2_TERRAFORM_APPLY_AUTO_APPROVE` | `1` | Set to `0` for interactive `apply` (no `-auto-approve`). |
| `R2_TERRAFORM_VALIDATE_ARGS` | *(empty)* | Extra args for `terraform validate`. |
| `R2_TERRAFORM_FMT_ARGS` | *(empty)* | Extra args for `terraform fmt`. |
| `R2_TERRAFORM_IMPORT_ARGS` | *(empty)* | Single `terraform import` (tokens after `terraform import -input=false`). |
| `R2_TERRAFORM_IMPORT_FILE` | *(empty)* | File of imports: one line per import, `ADDRESS` + space + `ID` (rest of line). |
| `R2_PUBLISH_ALWAYS_UPLOAD` | `0` | Set to `1` to run tarball upload even when `apply` is not in `R2_TERRAFORM_COMMANDS` (e.g. after `init,plan` only). |

If `apply` is **not** in `R2_TERRAFORM_COMMANDS`, the script **skips** creating the tarball and uploading (unless `R2_PUBLISH_ALWAYS_UPLOAD=1`).

**Examples (Terraform via `dockpipe.cloudflare.r2infra`):**

```bash
# Plan only (init auto-prepended)
export R2_TERRAFORM_COMMANDS=plan
dockpipe --workflow dockpipe.cloudflare.r2infra

# Init + apply (default commands)
dockpipe --workflow dockpipe.cloudflare.r2infra
```

### Terraform: public hostname, WAF, CDN (optional)

Pass [Terraform input variables](https://developer.hashicorp.com/terraform/language/values/variables) for the module in **`terraform/`** via **`terraform.tfvars`** or **`TF_VAR_*`**. Flags include **`enable_r2_custom_domain`**, **`enable_waf_baseline`**, **`enable_cache_rules`**, plus **`zone_id`** and **`public_hostname`**. Names, phases, Rules expressions, and TTL modes are all variables — start from **`terraform/terraform.tfvars.example`** — see **`terraform/README.md`**.

Example:

```bash
export TF_VAR_zone_id="..."
export TF_VAR_public_hostname="cdn.example.com"
export TF_VAR_enable_r2_custom_domain=true
export TF_VAR_enable_waf_baseline=true
export TF_VAR_enable_cache_rules=true
cd .staging/workflows/dockpipe.storage.cloudflare.r2/dockpipe.cloudflare.r2publish/terraform   # or your copy
terraform init    # configure backend per terraform/README.md
terraform apply
```

**Terraform:** With **S3 keys**, Terraform is **off** unless you set **`R2_USE_TERRAFORM=1`**. With **`CLOUDFLARE_API_TOKEN`** only, Terraform runs **by default** unless **`R2_SKIP_TERRAFORM=1`**.

**Remote state:** Defaults match a shared **`dockpipe`** bucket with a **`state/`** prefix in the dashboard. Override bucket/key with **`R2_TF_STATE_*`** if you use a different layout.

## Git

- **`release/artifacts/`** is gitignored — put build artifacts there, then publish.
- Do **not** commit API tokens; use env or a secret manager.
- Local Terraform state under the module dir (`*.tfstate`, `.terraform/`) is gitignored when using **`R2_TF_BACKEND=local`**. With **remote** state, the live state lives in R2; **commit** `terraform/.terraform.lock.hcl` for reproducible provider versions.
