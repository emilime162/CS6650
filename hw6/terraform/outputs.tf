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