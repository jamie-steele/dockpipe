locals {
  # Provider accepts api_token OR legacy email + Global API Key (not both).
  use_cloudflare_api_token = trimspace(var.cloudflare_api_token) != ""
}

provider "cloudflare" {
  api_token = local.use_cloudflare_api_token ? var.cloudflare_api_token : null
  email     = local.use_cloudflare_api_token ? null : var.cloudflare_email
  api_key   = local.use_cloudflare_api_token ? null : var.cloudflare_api_key
}

resource "cloudflare_r2_bucket" "publish" {
  account_id = var.account_id
  name       = var.bucket_name
  location   = var.location != "" ? var.location : null
}
