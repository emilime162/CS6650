output "table_name" {
  value = aws_dynamodb_table.carts.name
}

output "table_arn" {
  value = aws_dynamodb_table.carts.arn
}