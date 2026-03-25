terraform {
  required_version = ">= 1.0"

  # Remote state on Cloudflare R2 (S3-compatible). Full config is supplied at `terraform init`
  # via `-backend-config` (see r2-publish README and r2-publish.sh); defaults: bucket `dockpipe`,
  # object key `state/r2-publish/terraform.tfstate`.
  backend "s3" {
  }

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.0"
    }
  }
}
