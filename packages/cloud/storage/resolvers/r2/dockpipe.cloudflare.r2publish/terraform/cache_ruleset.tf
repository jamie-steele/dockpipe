# Edge cache for static package artifacts (high read volume). Scoped to public_hostname and
# common package suffixes unless cache_rule_expression is overridden.
resource "cloudflare_ruleset" "package_cdn_cache" {
  count = var.enable_cache_rules ? 1 : 0

  zone_id     = var.zone_id
  name        = var.cache_ruleset_name
  description = var.cache_ruleset_description
  kind        = "zone"
  phase       = var.cache_ruleset_phase

  rules = [
    {
      ref         = var.cache_rule_ref
      description = var.cache_rule_description
      expression  = local.cache_expression
      enabled     = var.cache_rule_enabled
      action      = "set_cache_settings"
      action_parameters = {
        cache = var.cache_eligible
        edge_ttl = {
          mode    = var.cache_edge_ttl_mode
          default = var.cache_edge_ttl_seconds
        }
        browser_ttl = {
          mode    = var.cache_browser_ttl_mode
          default = var.cache_browser_ttl_seconds
        }
        respect_strong_etags = var.cache_respect_strong_etags
      }
    }
  ]

  lifecycle {
    precondition {
      condition     = var.zone_id != "" && var.public_hostname != ""
      error_message = "enable_cache_rules requires non-empty zone_id and public_hostname."
    }
  }
}
