output "bucket_name" {
  description = "R2 bucket name."
  value       = cloudflare_r2_bucket.publish.name
}

output "account_id" {
  description = "Cloudflare account ID."
  value       = var.account_id
}
