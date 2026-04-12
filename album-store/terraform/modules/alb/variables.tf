variable "name_prefix" {
  description = "Prefix for resource names"
  type        = string
  default     = "album-store"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "dev"
}

variable "vpc_id" {
  description = "VPC ID"
  type        = string
}
