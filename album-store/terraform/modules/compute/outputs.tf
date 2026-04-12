output "instance_ids" {
  description = "IDs of the EC2 instances"
  value       = aws_instance.album_store[*].id
}

output "instance_public_ips" {
  description = "Public IPs of the EC2 instances"
  value       = aws_instance.album_store[*].public_ip
}

output "instance_public_dns" {
  description = "Public DNS of the EC2 instances"
  value       = aws_instance.album_store[*].public_dns
}

output "instance_count" {
  description = "Number of instances created"
  value       = length(aws_instance.album_store)
}
