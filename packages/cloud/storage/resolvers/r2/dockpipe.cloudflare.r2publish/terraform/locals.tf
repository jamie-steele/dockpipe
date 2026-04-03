locals {
  # Host filter: custom expression, or http.host eq public_hostname.
  host_filter = var.edge_host_filter_expression != "" ? var.edge_host_filter_expression : (
    var.public_hostname != "" ? "(http.host eq \"${var.public_hostname}\")" : "false"
  )

  # WAF: full override, else host filter.
  waf_expression = var.waf_rule_expression != "" ? var.waf_rule_expression : local.host_filter

  waf_rule_description_rendered = replace(var.waf_rule_description, "{hostname}", var.public_hostname)

  # Default cache expression: host + OR of path suffixes. Empty suffix list => match any path on that host (see README).
  path_suffix_clause = length(var.cache_path_suffixes) > 0 ? join(" or ", [
    for s in var.cache_path_suffixes : "ends_with(http.request.uri.path, \"${s}\")"
  ]) : "true"

  default_cache_expression = "${local.host_filter} and (${local.path_suffix_clause})"

  cache_expression = var.cache_rule_expression != "" ? var.cache_rule_expression : local.default_cache_expression
}
