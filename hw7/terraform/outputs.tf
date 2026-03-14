output "ecs_cluster_name" {
  description = "Name of the created ECS cluster"
  value       = module.ecs.cluster_name
}

output "ecs_service_name" {
  description = "Name of the running ECS service"
  value       = module.ecs.service_name
}

output "alb_dns_name" {
  description = "Use this as your Locust --host"
  value       = module.alb.alb_dns_name
}

output "sns_topic_arn" {
  description = "SNS topic ARN — for debugging/verification"
  value       = module.messaging.sns_topic_arn
}

output "sqs_queue_url" {
  description = "SQS queue URL — for debugging/verification"
  value       = module.messaging.sqs_queue_url
}