# dockpipe.cloudflare.r2publish — Terraform module

Creates an R2 bucket via `cloudflare_r2_bucket` (when you are not importing an existing bucket).

Optionally (feature flags, **all default off**):

- **`cloudflare_r2_custom_domain`** — public HTTPS hostname on your Cloudflare zone, bound to the bucket.
- **`cloudflare_ruleset`** in **`http_request_firewall_managed`** — runs the **Cloudflare Managed Ruleset** for traffic to that hostname only (baseline WAF).
- **`cloudflare_ruleset`** in **`http_request_cache_settings`** — edge + browser cache TTLs for package-like paths (`.tar.gz`, `.sha256`, `.json`) on that hostname (CDN).

Requires **Cloudflare Terraform provider `~> 5.0`** (see `versions.tf` and `.terraform.lock.hcl`).

## Variables (edge / optional)

| Variable | Default | Meaning |
|----------|---------|---------|
| `zone_id` | `""` | Cloudflare **zone ID** for the domain that will serve `public_hostname`. |
| `public_hostname` | `""` | FQDN, e.g. `cdn.example.com` (must be on `zone_id`). |
| `enable_r2_custom_domain` | `false` | Create `cloudflare_r2_custom_domain` for `public_hostname` → bucket. |
| `r2_custom_domain_enabled` | `true` | Maps to `cloudflare_r2_custom_domain.enabled`. |
| `r2_custom_domain_min_tls` | `"1.2"` | Minimum TLS version for the R2 custom domain. |
| `enable_waf_baseline` | `false` | Deploy managed WAF ruleset for `http.host eq public_hostname`. |
| `cloudflare_managed_ruleset_id` | (see `variables.tf`) | **Execute** target ID for Cloudflare Managed Ruleset; override if Cloudflare changes catalog IDs. |
| `enable_cache_rules` | `false` | Deploy cache rules for package artifacts on that hostname. |
| `cache_rule_expression` | `""` | Override the default Rules expression (host + path suffixes). |
| `cache_edge_ttl_seconds` | `86400` | Edge cache TTL (override origin). |
| `cache_browser_ttl_seconds` | `3600` | Browser cache TTL. |

### Customization (forks / white-label)

All ruleset **names**, **descriptions**, **refs**, **phases**, **rule enabled** flags, **TTL modes**, and **Rules expressions** are variables — copy the module and override in **`terraform.tfvars`** without editing `.tf` files.

| Variable | Purpose |
|----------|---------|
| `edge_host_filter_expression` | Replace the default `(http.host eq "…")` filter (multi-host, advanced Rules). |
| `waf_rule_expression` | Full override for the WAF rule expression (ignores `edge_host_filter_expression` for WAF). |
| `cache_path_suffixes` | List of path suffixes OR’d for the **default** cache expression (when `cache_rule_expression` is empty). Empty list ⇒ path clause is `true` (entire hostname). |
| `waf_ruleset_name`, `waf_ruleset_description`, `waf_ruleset_phase` | WAF `cloudflare_ruleset` metadata. |
| `waf_rule_ref`, `waf_rule_description`, `waf_rule_enabled` | Single WAF rule; description may include `{hostname}` → replaced with `public_hostname`. |
| `cache_ruleset_name`, `cache_ruleset_description`, `cache_ruleset_phase` | Cache ruleset metadata. |
| `cache_rule_ref`, `cache_rule_description`, `cache_rule_enabled` | Single cache rule. |
| `cache_edge_ttl_mode`, `cache_browser_ttl_mode` | e.g. `override_origin` vs `respect_origin`. |
| `cache_respect_strong_etags`, `cache_eligible` | Fine-tune `set_cache_settings`. |

See **`variables.tf`** for defaults and **`terraform.tfvars.example`** for a commented template.

Set via **`terraform.tfvars`**, **`*.auto.tfvars`**, or **`TF_VAR_*`** (see [Terraform variables](https://developer.hashicorp.com/terraform/language/values/variables)).

Example **`terraform.tfvars`** (do not commit secrets):

```hcl
zone_id                 = "your_zone_id"
public_hostname         = "cdn.example.com"
enable_r2_custom_domain = true
enable_waf_baseline     = true
enable_cache_rules      = true

# Optional white-label names:
# waf_ruleset_name    = "acme-cdn-waf"
# cache_ruleset_name  = "acme-cdn-cache"
```

### API token permissions (single token for `terraform apply`)

Broaden the Cloudflare API token beyond **Account → R2 → Edit** when using edge resources:

- **Account** — R2: Edit (bucket + custom domain).
- **Zone** — DNS (and SSL if prompted for custom hostname), **WAF**, **Cache Rules** (wording in the dashboard may be “Zone” / “Rules” / “Firewall”).

If Terraform returns **permission denied**, add the missing zone permissions for the token.

### Plan limits

WAF managed rules and cache rules availability varies by **Cloudflare plan** (rule counts, features). If `apply` fails on a ruleset, check the error for plan upgrades or reduce scope.

### One ruleset per phase

Cloudflare allows **one zone-level entry ruleset** per [phase](https://developers.cloudflare.com/ruleset-engine/about/phases/) in some setups. If the zone already has a custom ruleset in **`http_request_firewall_managed`** or **`http_request_cache_settings`**, either:

- **Import** the existing ruleset into Terraform and merge rules, or  
- Manage that phase outside Terraform and **do not** enable `enable_waf_baseline` / `enable_cache_rules` here.

### Import an existing R2 custom domain

If the domain binding already exists in the dashboard, import before apply (ID format is provider-specific; confirm with `terraform plan` after `terraform import` for your provider version):

```bash
# Example — verify resource address and ID in provider docs for your version.
terraform import 'cloudflare_r2_custom_domain.publish[0]' '<IMPORT_ID>'
```

## Remote state on R2

State is stored in the **`dockpipe`** bucket (override with `R2_TF_STATE_BUCKET` / `R2_TF_STATE_KEY` in **`r2-publish.sh`**) under the object key **`state/dockpipe.cloudflare.r2publish/terraform.tfstate`** (legacy: **`state/r2-publish/terraform.tfstate`**), using Terraform’s **`s3`** backend and [R2’s S3 API](https://developers.cloudflare.com/terraform/advanced-topics/remote-backend/).

The S3 backend needs **R2 API tokens** (Access Key ID + Secret Access Key) scoped to that bucket with **Object Read & Write** — not the Cloudflare API token used by the Terraform provider. Set **`R2_STATE_ACCESS_KEY_ID`** and **`R2_STATE_SECRET_ACCESS_KEY`**, or reuse **`AWS_ACCESS_KEY_ID`** / **`AWS_SECRET_ACCESS_KEY`** if they already target that bucket.

For **local state only** (no R2 backend), run with **`R2_TF_BACKEND=local`** (see workflow README).

## Import an existing bucket

If the bucket named in **`R2_BUCKET`** already exists (for example you created it in the dashboard), import it before the first apply:

```bash
cd workflows/dockpipe.cloudflare.r2publish/terraform   # or your copy under templates/dockpipe.cloudflare.r2publish/terraform
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
