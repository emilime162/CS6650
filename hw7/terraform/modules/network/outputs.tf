output "vpc_id" {
  value = aws_vpc.this.id
}

output "public_subnet_ids" {
  value = [aws_subnet.public_1.id, aws_subnet.public_2.id]
}

output "private_subnet_ids" {
  value = [aws_subnet.private_1.id, aws_subnet.private_2.id]
}

# kept for backward compat with alb module
output "subnet_ids" {
  value = [aws_subnet.public_1.id, aws_subnet.public_2.id]
}

output "security_group_id" {
  value = aws_security_group.this.id
}

output "alb_security_group_id" {
  value = aws_security_group.alb.id
}