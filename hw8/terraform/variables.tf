variable "aws_region" {
  type    = string
  default = "us-west-2"
}

variable "ecr_repository_name" {
  type    = string
  default = "ecr_service"
}

variable "service_name" {
  type    = string
  default = "CS6650L2"
}

variable "container_port" {
  type    = number
  default = 8080
}

variable "ecs_count" {
  type    = number
  default = 1
}

variable "log_retention_days" {
  type    = number
  default = 7
}

variable "db_password" {
  type      = string
  sensitive = true
  default   = "Password123!"
}