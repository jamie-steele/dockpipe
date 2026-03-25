# r2-publish

Host workflow: **tar.gz** a local folder (default **`./release/artifacts`**, gitignored) and upload to **Cloudflare R2**.

## Two ways to authenticate

| Mode | Credentials | Upload | Bucket |
|------|-------------|--------|--------|
| **S3 API** | `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` (R2 access keys) | `aws s3 cp` | You create the bucket (or use Terraform separately). |
| **Single API token** | `CLOUDFLARE_API_TOKEN` + `CLOUDFLARE_ACCOUNT_ID` | Terraform **creates** the bucket, then **`wrangler r2 object put`** | Created by Terraform unless you skip it. |

**Terraform state** defaults to **remote** on R2: bucket **`dockpipe`**, object key **`state/r2-publish/terraform.tfstate`** (same account as `CLOUDFLARE_ACCOUNT_ID`). The S3-compatible backend uses **R2 access keys** (`R2_STATE_*` or `AWS_*`), not the Cloudflare API token — see [Remote R2 backend](https://developers.cloudflare.com/terraform/advanced-topics/remote-backend/). If the publish bucket already exists, **`terraform import`** before apply — see **`terraform/README.md`**.

R2 is the **destination**, not a DockPipe runtime. **`cloudflare`** is not a runtime; the runtime here is **`cli`** on the host (`skip_container: true`).

**Consumers:** After you publish **`release/artifacts/`** (or a custom prefix), point a **public HTTPS hostname** (Cloudflare **Custom Domain** on the bucket or a Worker) at the same objects. Downstream projects can run **`dockpipe install core --base-url https://your-cdn.example/dockpipe`** (or **`DOCKPIPE_INSTALL_BASE_URL`**) to replace **`templates/core/`** without cloning the full dockpipe repo. Build artifacts with **`make package-templates-core`** or **`dockpipe package build core`** at the repo root.

## Prerequisites

- **`release/artifacts/`** (or `R2_PUBLISH_SOURCE`) with something to pack
- **`R2_BUCKET`** — bucket name (Terraform uses this name when creating the bucket)

### S3 mode

- **aws** CLI v2
- **Endpoint** — `R2_ENDPOINT_URL=https://<account_id>.r2.cloudflarestorage.com` or **`CLOUDFLARE_ACCOUNT_ID`** so the script can build the URL

See [Cloudflare R2 — AWS CLI](https://developers.cloudflare.com/r2/examples/aws/aws-cli/).

### Single-token mode (Terraform + Wrangler)

- **Terraform** (`terraform` on `PATH`) — runs **`workflows/r2-publish/terraform`** (or `R2_TERRAFORM_DIR`) before upload
- **Wrangler** — either `wrangler` on `PATH`, or **Node.js** (`npx` runs `wrangler@3`)
- **API token** with at least **Account → R2 → Edit** (bucket + objects). See [R2 API tokens](https://developers.cloudflare.com/r2/api/tokens/).
- **R2 API keys for Terraform state** — scoped to the **`dockpipe`** bucket (Object Read & Write). Set **`R2_STATE_ACCESS_KEY_ID`** / **`R2_STATE_SECRET_ACCESS_KEY`**, or use the same **`AWS_*`** pair if it already targets that bucket. Use **`R2_TF_BACKEND=local`** to keep state only on disk (no keys).

Terraform module: [Cloudflare provider + `cloudflare_r2_bucket`](https://developers.cloudflare.com/r2/examples/terraform/) (provider lock file: `terraform/.terraform.lock.hcl`). **Import** existing buckets: **`terraform/README.md`**.

## Run (this repo)

### S3 keys

```bash
make build   # if needed
export R2_BUCKET=your-bucket
export CLOUDFLARE_ACCOUNT_ID=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
./src/bin/dockpipe --workflow r2-publish --workdir . --
```

### Single Cloudflare API token — bucket + upload

```bash
export R2_BUCKET=your-bucket
export CLOUDFLARE_ACCOUNT_ID=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
export CLOUDFLARE_API_TOKEN=...
./src/bin/dockpipe --workflow r2-publish --workdir . --
```

Dry run (no Terraform apply, no upload):

```bash
R2_PUBLISH_DRY_RUN=1 ./src/bin/dockpipe --workflow r2-publish --workdir . --
```

## Environment variables

| Variable | Default | Meaning |
|----------|---------|---------|
| `R2_PUBLISH_SOURCE` | `release/artifacts` | Directory to tar (relative to workdir). |
| `R2_PREFIX` | *(empty)* | Key prefix in the bucket, e.g. `releases/` |
| `R2_ARCHIVE_NAME` | `dockpipe-publish-YYYYMMDD-HHMMSS.tar.gz` | Object name in the bucket. |
| `R2_ENDPOINT_URL` | *(from account id, S3 mode)* | Override S3 endpoint URL. |
| `R2_CONTENT_TYPE` | `application/gzip` | Wrangler object `Content-Type` (token mode). |
| `R2_TERRAFORM_DIR` | *(auto)* | Path to Terraform module (default: `workflows/r2-publish/terraform` or `templates/r2-publish/terraform` under workdir). |
| `R2_TF_LOCATION` | *(omit)* | Optional `location` for `cloudflare_r2_bucket` (e.g. `WEUR`). |
| `R2_SKIP_TERRAFORM` | `0` | Set to `1` to skip `terraform apply` (bucket must already exist). |
| `R2_USE_TERRAFORM` | *(see below)* | `1` to always run Terraform; `0` to never run. |
| `R2_PUBLISH_DRY_RUN` | `0` | Set to `1` to print only what would run. |
| `R2_TF_BACKEND` | `remote` | `remote` — state in R2 (`dockpipe` / `state/r2-publish/terraform.tfstate` by default). `local` — `terraform init -backend=false` (no R2 keys). |
| `R2_TF_STATE_BUCKET` | `dockpipe` | State object bucket (S3 backend). |
| `R2_TF_STATE_KEY` | `state/r2-publish/terraform.tfstate` | State object key inside that bucket. |
| `R2_STATE_ACCESS_KEY_ID` | *(unset)* | R2 S3 API access key for the **state** bucket (or use `AWS_ACCESS_KEY_ID` when it targets that bucket). |
| `R2_STATE_SECRET_ACCESS_KEY` | *(unset)* | Secret for `R2_STATE_ACCESS_KEY_ID`. |

**Terraform:** With **S3 keys**, Terraform is **off** unless you set **`R2_USE_TERRAFORM=1`**. With **`CLOUDFLARE_API_TOKEN`** only, Terraform runs **by default** unless **`R2_SKIP_TERRAFORM=1`**.

**Remote state:** Defaults match a shared **`dockpipe`** bucket with a **`state/`** prefix in the dashboard. Override bucket/key with **`R2_TF_STATE_*`** if you use a different layout.

## Git

- **`release/artifacts/`** is gitignored — put build artifacts there, then publish.
- Do **not** commit API tokens; use env or a secret manager.
- Local Terraform state under the module dir (`*.tfstate`, `.terraform/`) is gitignored when using **`R2_TF_BACKEND=local`**. With **remote** state, the live state lives in R2; **commit** `terraform/.terraform.lock.hcl` for reproducible provider versions.
