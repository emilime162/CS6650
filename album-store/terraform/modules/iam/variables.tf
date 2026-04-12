variable "name_prefix" {
  description = "Prefix for IAM resource names"
  type        = string
  default     = "album-store"
}

variable "environment" {
  description = "Environment name (e.g., dev, prod)"
  type        = string
  default     = "dev"
}

variable "albums_table_arn" {
  description = "ARN of the albums DynamoDB table"
  type        = string
}

variable "photos_table_arn" {
  description = "ARN of the photos DynamoDB table"
  type        = string
}

variable "s3_bucket_arn" {
  description = "ARN of the S3 bucket"
  type        = string
}
