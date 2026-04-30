terraform {
  # resource lifecycle preconditions (edge modules) require >= 1.2
  required_version = ">= 1.2"

  # Remote state on Cloudflare R2 (S3-compatible). Full config is supplied at `terraform init`
  # via `-backend-config` (see workflow README and r2-publish.sh); defaults: bucket `dockpipe`,
  # object key `state/dockpipe.cloudflare.r2publish/terraform.tfstate`.
  backend "s3" {
  }

  required_providers {
    cloudflare = {
      source = "cloudflare/cloudflare"
      # 5.x adds cloudflare_r2_custom_domain and current ruleset schemas for cache/WAF.
      version = "~> 5.0"
    }
  }
}
