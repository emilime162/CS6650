output "albums_table_name" {
  description = "Name of the albums DynamoDB table"
  value       = aws_dynamodb_table.albums.name
}

output "albums_table_arn" {
  description = "ARN of the albums DynamoDB table"
  value       = aws_dynamodb_table.albums.arn
}

output "photos_table_name" {
  description = "Name of the photos DynamoDB table"
  value       = aws_dynamodb_table.photos.name
}

output "photos_table_arn" {
  description = "ARN of the photos DynamoDB table"
  value       = aws_dynamodb_table.photos.arn
}

output "s3_bucket_name" {
  description = "Name of the S3 bucket"
  value       = aws_s3_bucket.photos.id
}

output "s3_bucket_arn" {
  description = "ARN of the S3 bucket"
  value       = aws_s3_bucket.photos.arn
}

output "s3_bucket_domain_name" {
  description = "Domain name of the S3 bucket"
  value       = aws_s3_bucket.photos.bucket_domain_name
}
