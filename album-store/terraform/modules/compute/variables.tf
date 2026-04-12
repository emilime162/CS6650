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

variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "t3.xlarge"
}

variable "key_name" {
  description = "Name of the SSH key pair"
  type        = string
}

variable "iam_instance_profile" {
  description = "Name of the IAM instance profile"
  type        = string
}

variable "security_group_id" {
  description = "ID of the security group"
  type        = string
}

variable "aws_region" {
  description = "AWS region"
  type        = string
}

variable "albums_table_name" {
  description = "Name of the albums DynamoDB table"
  type        = string
}

variable "photos_table_name" {
  description = "Name of the photos DynamoDB table"
  type        = string
}

variable "s3_bucket_name" {
  description = "Name of the S3 bucket"
  type        = string
}

variable "go_version" {
  description = "Go version to install"
  type        = string
  default     = "1.21.6"
}

variable "git_repo_url" {
  description = "Git repository URL (optional, for automated deployment)"
  type        = string
  default     = ""
}

variable "git_branch" {
  description = "Git branch to checkout (if using git_repo_url)"
  type        = string
  default     = "main"
}

variable "root_volume_size" {
  description = "Size of the root EBS volume in GB"
  type        = number
  default     = 30
}

variable "instance_count" {
  description = "Number of EC2 instances to create"
  type        = number
  default     = 1
}

variable "target_group_arn" {
  description = "ARN of the target group (for ALB)"
  type        = string
  default     = ""
}
