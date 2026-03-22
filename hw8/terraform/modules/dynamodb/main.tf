resource "aws_dynamodb_table" "carts" {
  name         = "${var.service_name}-carts"
  billing_mode = "PAY_PER_REQUEST"  # no capacity planning needed
  hash_key     = "cart_id"

  attribute {
    name = "cart_id"
    type = "S"  # String
  }

  tags = { Name = "${var.service_name}-carts" }
}