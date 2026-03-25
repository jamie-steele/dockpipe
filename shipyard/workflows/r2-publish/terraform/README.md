# r2-publish Terraform module

Creates an R2 bucket via `cloudflare_r2_bucket` (when you are not importing an existing bucket).

## Remote state on R2

State is stored in the **`dockpipe`** bucket (override with `R2_TF_STATE_BUCKET` / `R2_TF_STATE_KEY` in **`r2-publish.sh`**) under the object key **`state/r2-publish/terraform.tfstate`**, using Terraform’s **`s3`** backend and [R2’s S3 API](https://developers.cloudflare.com/terraform/advanced-topics/remote-backend/).

The S3 backend needs **R2 API tokens** (Access Key ID + Secret Access Key) scoped to that bucket with **Object Read & Write** — not the Cloudflare API token used by the Terraform provider. Set **`R2_STATE_ACCESS_KEY_ID`** and **`R2_STATE_SECRET_ACCESS_KEY`**, or reuse **`AWS_ACCESS_KEY_ID`** / **`AWS_SECRET_ACCESS_KEY`** if they already target that bucket.

For **local state only** (no R2 backend), run with **`R2_TF_BACKEND=local`** (see workflow README).

## Import an existing bucket

If the bucket named in **`R2_BUCKET`** already exists (for example you created it in the dashboard), import it before the first apply:

```bash
cd shipyard/workflows/r2-publish/terraform   # or your copy under templates/r2-publish/terraform
terraform init    # after configuring backend (see workflow README)
terraform import 'cloudflare_r2_bucket.publish' '<ACCOUNT_ID>/<BUCKET_NAME>'
```

Use the same **account ID** and **bucket name** as in **`CLOUDFLARE_ACCOUNT_ID`** and **`R2_BUCKET`**.

## Migrating local state to R2

After the remote backend is configured and credentials work:

```bash
terraform init -migrate-state
```

Follow the prompts to upload existing local state into the bucket.
