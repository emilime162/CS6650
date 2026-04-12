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
  description = "Name of the S3 bucket for photo storage"
  type        = string
}

variable "environment" {
  description = "Environment name (e.g., dev, prod)"
  type        = string
  default     = "dev"
}
