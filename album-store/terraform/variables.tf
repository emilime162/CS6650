# ─────────────────────────────────────────────────────────────────────────────
# Root Variables
# ─────────────────────────────────────────────────────────────────────────────

variable "aws_region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name (e.g., dev, prod)"
  type        = string
  default     = "dev"
}

variable "name_prefix" {
  description = "Prefix for resource names"
  type        = string
  default     = "album-store"
}

# ── Storage ───────────────────────────────────────────────────────────────────
variable "albums_table_name" {
  description = "Name of the DynamoDB table for albums"
  type        = string
  default     = "albums"
}

variable "photos_table_name" {
  description = "Name of the DynamoDB table for photos"
  type        = string
  default     = "photos"
}

variable "s3_bucket_name" {
  description = "Name of the S3 bucket (must be globally unique)"
  type        = string
}

# ── Networking ────────────────────────────────────────────────────────────────
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

# ── Compute ───────────────────────────────────────────────────────────────────
variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "t3.xlarge"
}

variable "instance_count" {
  description = "Number of EC2 instances behind ALB"
  type        = number
  default     = 3
}

variable "key_name" {
  description = "Name of the SSH key pair"
  type        = string
}

variable "go_version" {
  description = "Go version to install on EC2"
  type        = string
  default     = "1.21.6"
}

variable "git_repo_url" {
  description = "Git repository URL for automated deployment (leave empty for manual deployment)"
  type        = string
  default     = ""
}

variable "git_branch" {
  description = "Git branch to checkout (if using git_repo_url)"
  type        = string
  default     = "main"
}
