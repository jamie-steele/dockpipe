# Zone-level entry ruleset for the http_request_firewall_managed phase: executes the
# Cloudflare Managed Ruleset for traffic to public_hostname only.
# If this phase already has a ruleset in the zone, import it or merge rules — see terraform/README.md.
resource "cloudflare_ruleset" "waf_managed_r2_cdn" {
  count = var.enable_waf_baseline ? 1 : 0

  zone_id     = var.zone_id
  name        = var.waf_ruleset_name
  description = var.waf_ruleset_description
  kind        = "zone"
  phase       = var.waf_ruleset_phase

  rules = [
    {
      ref         = var.waf_rule_ref
      description = local.waf_rule_description_rendered
      expression  = local.waf_expression
      enabled     = var.waf_rule_enabled
      action      = "execute"
      action_parameters = {
        id = var.cloudflare_managed_ruleset_id
      }
    }
  ]

  lifecycle {
    precondition {
      condition     = var.zone_id != "" && var.public_hostname != ""
      error_message = "enable_waf_baseline requires non-empty zone_id and public_hostname."
    }
  }
}
