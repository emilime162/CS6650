output "security_group_id" {
  description = "ID of the security group"
  value       = aws_security_group.album_store.id
}

output "security_group_name" {
  description = "Name of the security group"
  value       = aws_security_group.album_store.name
}

output "vpc_id" {
  description = "ID of the VPC"
  value       = data.aws_vpc.default.id
}
