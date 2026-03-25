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
