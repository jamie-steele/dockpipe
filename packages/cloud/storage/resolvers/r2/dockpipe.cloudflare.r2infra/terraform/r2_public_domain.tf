resource "cloudflare_r2_custom_domain" "publish" {
  count = var.enable_r2_custom_domain ? 1 : 0

  account_id  = var.account_id
  bucket_name = cloudflare_r2_bucket.publish.name
  domain      = var.public_hostname
  zone_id     = var.zone_id
  enabled     = var.r2_custom_domain_enabled
  min_tls     = var.r2_custom_domain_min_tls

  lifecycle {
    precondition {
      condition     = var.zone_id != "" && var.public_hostname != ""
      error_message = "enable_r2_custom_domain requires non-empty zone_id and public_hostname."
    }
  }
}
