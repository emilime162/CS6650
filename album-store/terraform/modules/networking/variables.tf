variable "name_prefix" {
  description = "Prefix for resource names"
  type        = string
  default     = "album-store"
}

variable "environment" {
  description = "Environment name (e.g., dev, prod)"
  type        = string
  default     = "dev"
}

variable "allowed_cidr_blocks" {
  description = "CIDR blocks allowed to access port 8080"
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "ssh_cidr_blocks" {
  description = "CIDR blocks allowed to SSH (port 22)"
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "alb_security_group_id" {
  description = "Security group ID of the ALB (optional)"
  type        = string
  default     = ""
}
