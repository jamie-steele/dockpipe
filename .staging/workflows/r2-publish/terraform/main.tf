provider "cloudflare" {
  api_token = var.cloudflare_api_token
}

resource "cloudflare_r2_bucket" "publish" {
  account_id = var.account_id
  name       = var.bucket_name
  location   = var.location != "" ? var.location : null
}
