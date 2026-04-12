# ─────────────────────────────────────────────────────────────────────────────
# Root Outputs - Load Balanced Architecture
# ─────────────────────────────────────────────────────────────────────────────

# ── Storage ───────────────────────────────────────────────────────────────────
output "albums_table_name" {
  description = "Name of the albums DynamoDB table"
  value       = module.storage.albums_table_name
}

output "photos_table_name" {
  description = "Name of the photos DynamoDB table"
  value       = module.storage.photos_table_name
}

output "s3_bucket_name" {
  description = "Name of the S3 bucket"
  value       = module.storage.s3_bucket_name
}

# ── IAM ───────────────────────────────────────────────────────────────────────
output "iam_role_name" {
  description = "Name of the IAM role"
  value       = module.iam.role_name
}

output "instance_profile_name" {
  description = "Name of the EC2 instance profile"
  value       = module.iam.instance_profile_name
}

# ── Load Balancer ─────────────────────────────────────────────────────────────
output "alb_dns_name" {
  description = "DNS name of the load balancer"
  value       = module.alb.alb_dns_name
}

output "base_url" {
  description = "Base URL for the album-store API (via ALB)"
  value       = "http://${module.alb.alb_dns_name}"
}

# ── Compute ───────────────────────────────────────────────────────────────────
output "instance_ids" {
  description = "IDs of the EC2 instances"
  value       = module.compute.instance_ids
}

output "instance_public_ips" {
  description = "Public IPs of the EC2 instances"
  value       = module.compute.instance_public_ips
}

output "instance_count" {
  description = "Number of instances"
  value       = module.compute.instance_count
}

# ── Summary ───────────────────────────────────────────────────────────────────
output "deployment_summary" {
  description = "Summary of deployed resources"
  value = {
    api_endpoint     = "http://${module.alb.alb_dns_name}"
    health_check     = "http://${module.alb.alb_dns_name}/health"
    instance_count   = module.compute.instance_count
    instance_ips     = join(", ", module.compute.instance_public_ips)
    albums_table     = module.storage.albums_table_name
    photos_table     = module.storage.photos_table_name
    s3_bucket        = module.storage.s3_bucket_name
  }
}
