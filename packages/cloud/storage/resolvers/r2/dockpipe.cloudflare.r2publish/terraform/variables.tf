variable "cloudflare_api_token" {
  description = "Cloudflare API token (Account R2: Edit, or broader)."
  type        = string
  sensitive   = true
}

variable "account_id" {
  description = "Cloudflare account ID."
  type        = string
}

variable "bucket_name" {
  description = "R2 bucket name to create."
  type        = string
}

variable "location" {
  description = "Optional R2 location hint (e.g. WEUR, WNAM). Leave empty to omit."
  type        = string
  default     = ""
}

# --- Public HTTPS hostname (R2 custom domain) + edge (optional) ---

variable "zone_id" {
  description = "Cloudflare zone ID for the domain that will serve the R2 public hostname (DNS + rules). Required when any enable_* edge flag is true."
  type        = string
  default     = ""
}

variable "public_hostname" {
  description = "FQDN for public package URLs, e.g. cdn.example.com. Must be a hostname on the given zone_id. Used for R2 custom domain and WAF/cache rule scoping."
  type        = string
  default     = ""
}

variable "enable_r2_custom_domain" {
  description = "If true, create cloudflare_r2_custom_domain binding public_hostname to the bucket (HTTPS on the edge)."
  type        = bool
  default     = false
}

variable "r2_custom_domain_enabled" {
  description = "Whether the R2 custom domain should serve traffic (maps to cloudflare_r2_custom_domain.enabled)."
  type        = bool
  default     = true
}

variable "r2_custom_domain_min_tls" {
  description = "Minimum TLS for the R2 custom domain. Allowed: 1.0, 1.1, 1.2, 1.3."
  type        = string
  default     = "1.2"
}

variable "enable_waf_baseline" {
  description = "If true, deploy a zone ruleset in http_request_firewall_managed that executes the Cloudflare Managed Ruleset for traffic to public_hostname only."
  type        = bool
  default     = false
}

variable "cloudflare_managed_ruleset_id" {
  description = "Cloudflare Managed Ruleset ID (execute target). Override if Cloudflare changes the catalog ID."
  type        = string
  default     = "efb7b8c949ac4650a09736fc376e9aee"
}

variable "enable_cache_rules" {
  description = "If true, deploy cache rules (CDN) for package-like paths on public_hostname (see cache_rule_expression)."
  type        = bool
  default     = false
}

variable "cache_rule_expression" {
  description = "Rules expression for cache rule. Default: public hostname plus .tar.gz, .sha256, .json paths. Override for stricter matching."
  type        = string
  default     = ""
}

variable "cache_edge_ttl_seconds" {
  description = "Edge cache TTL (seconds) when overriding origin for matched package objects."
  type        = number
  default     = 86400
}

variable "cache_browser_ttl_seconds" {
  description = "Browser cache TTL (seconds) for matched package objects (override_origin)."
  type        = number
  default     = 3600
}

# --- Customization (template / forks): expressions, names, and rule tuning ---

variable "edge_host_filter_expression" {
  description = "If non-empty, used as the WAF rule expression and as the host part of the default cache expression (instead of http.host eq public_hostname). Use for multi-hostname or advanced Rules language."
  type        = string
  default     = ""
}

variable "waf_rule_expression" {
  description = "If non-empty, overrides the WAF managed-rule expression entirely (edge_host_filter_expression is ignored for WAF). If empty, uses edge_host_filter_expression or the default host filter from public_hostname."
  type        = string
  default     = ""
}

variable "cache_path_suffixes" {
  description = "Path suffixes for the default cache rule expression (OR’d with ends_with). Ignored when cache_rule_expression is set."
  type        = list(string)
  default     = [".tar.gz", ".sha256", ".json"]
}

variable "waf_ruleset_name" {
  description = "cloudflare_ruleset name for the WAF phase."
  type        = string
  default     = "dockpipe-r2-waf-managed"
}

variable "waf_ruleset_description" {
  description = "cloudflare_ruleset description for the WAF phase."
  type        = string
  default     = "Execute Cloudflare Managed Ruleset for R2 package CDN hostname"
}

variable "waf_ruleset_phase" {
  description = "Ruleset phase for managed WAF (Cloudflare Ruleset Engine phase string)."
  type        = string
  default     = "http_request_firewall_managed"
}

variable "waf_rule_ref" {
  description = "Stable ref for the WAF rule (Terraform / Rules engine)."
  type        = string
  default     = "managed_rules_r2_host"
}

variable "waf_rule_description" {
  description = "Description on the WAF rule. Use {hostname} in the string to substitute public_hostname (e.g. \"WAF for {hostname}\")."
  type        = string
  default     = "Execute Cloudflare Managed Ruleset for {hostname}"
}

variable "waf_rule_enabled" {
  description = "Whether the WAF execute rule is enabled."
  type        = bool
  default     = true
}

variable "cache_ruleset_name" {
  description = "cloudflare_ruleset name for cache (CDN) rules."
  type        = string
  default     = "dockpipe-r2-package-cache"
}

variable "cache_ruleset_description" {
  description = "cloudflare_ruleset description for cache rules."
  type        = string
  default     = "CDN cache rules for package tarballs and manifests on R2 hostname"
}

variable "cache_ruleset_phase" {
  description = "Ruleset phase for cache settings."
  type        = string
  default     = "http_request_cache_settings"
}

variable "cache_rule_ref" {
  description = "Stable ref for the cache rule."
  type        = string
  default     = "package_cache_static"
}

variable "cache_rule_description" {
  description = "Description on the cache rule."
  type        = string
  default     = "Edge + browser TTL for package objects"
}

variable "cache_rule_enabled" {
  description = "Whether the cache rule is enabled."
  type        = bool
  default     = true
}

variable "cache_edge_ttl_mode" {
  description = "Edge TTL mode for the cache rule (e.g. override_origin, respect_origin, bypass_by_default)."
  type        = string
  default     = "override_origin"
}

variable "cache_browser_ttl_mode" {
  description = "Browser TTL mode for the cache rule (e.g. override_origin, respect_origin, bypass)."
  type        = string
  default     = "override_origin"
}

variable "cache_respect_strong_etags" {
  description = "Set respect_strong_etags on the cache rule action."
  type        = bool
  default     = true
}

variable "cache_eligible" {
  description = "action_parameters.cache — whether responses may be cached at the edge (subject to other settings)."
  type        = bool
  default     = true
}
