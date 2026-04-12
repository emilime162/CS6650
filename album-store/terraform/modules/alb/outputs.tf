output "alb_dns_name" {
  description = "DNS name of the load balancer"
  value       = aws_lb.album_store.dns_name
}

output "alb_arn" {
  description = "ARN of the load balancer"
  value       = aws_lb.album_store.arn
}

output "target_group_arn" {
  description = "ARN of the target group"
  value       = aws_lb_target_group.album_store.arn
}

output "alb_security_group_id" {
  description = "Security group ID of the ALB"
  value       = aws_security_group.alb.id
}
