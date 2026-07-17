output "bucket_name" {
  description = "R2 bucket name."
  value       = cloudflare_r2_bucket.publish.name
}

output "account_id" {
  description = "Cloudflare account ID."
  value       = var.account_id
}

output "public_base_url" {
  description = "HTTPS base URL for the public package hostname (when public_hostname is set)."
  value       = var.public_hostname != "" ? "https://${var.public_hostname}" : null
}

output "r2_custom_domain_name" {
  description = "Hostname bound to the bucket (if enable_r2_custom_domain)."
  value       = var.enable_r2_custom_domain ? cloudflare_r2_custom_domain.publish[0].domain : null
}

output "r2_custom_domain_status" {
  description = "R2 custom domain status (ownership + SSL), if created."
  value       = var.enable_r2_custom_domain ? cloudflare_r2_custom_domain.publish[0].status : null
}

output "waf_ruleset_id" {
  description = "Zone ruleset id for WAF managed entry (if enable_waf_baseline)."
  value       = var.enable_waf_baseline ? cloudflare_ruleset.waf_managed_r2_cdn[0].id : null
}

output "cache_ruleset_id" {
  description = "Zone ruleset id for package cache rules (if enable_cache_rules)."
  value       = var.enable_cache_rules ? cloudflare_ruleset.package_cdn_cache[0].id : null
}

output "edge_host_filter_effective" {
  description = "Resolved host filter (edge_host_filter_expression or default from public_hostname)."
  value       = local.host_filter
}

output "waf_rule_expression_effective" {
  description = "Expression used for the WAF execute rule."
  value       = var.enable_waf_baseline ? local.waf_expression : null
}

output "cache_rule_expression_effective" {
  description = "Expression used for the cache rule."
  value       = var.enable_cache_rules ? local.cache_expression : null
}
