# ─────────────────────────────────────────────────────────────────────────────
# IAM Module: EC2 instance role with DynamoDB and S3 permissions
# ─────────────────────────────────────────────────────────────────────────────

# ── IAM role for EC2 instance ─────────────────────────────────────────────────
resource "aws_iam_role" "ec2_role" {
  name = "${var.name_prefix}-ec2-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })

  tags = {
    Name        = "${var.name_prefix}-ec2-role"
    Environment = var.environment
    Project     = "album-store"
  }
}

# ── IAM policy for DynamoDB and S3 access ─────────────────────────────────────
resource "aws_iam_role_policy" "album_store_policy" {
  name = "${var.name_prefix}-album-store-policy"
  role = aws_iam_role.ec2_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "DynamoDBAccess"
        Effect = "Allow"
        Action = [
          "dynamodb:PutItem",
          "dynamodb:GetItem",
          "dynamodb:UpdateItem",
          "dynamodb:DeleteItem",
          "dynamodb:Query",
          "dynamodb:Scan",
          "dynamodb:BatchGetItem",
          "dynamodb:BatchWriteItem"
        ]
        Resource = [
          var.albums_table_arn,
          var.photos_table_arn
        ]
      },
      {
        Sid    = "S3Access"
        Effect = "Allow"
        Action = [
          "s3:PutObject",
          "s3:GetObject",
          "s3:DeleteObject",
          "s3:ListBucket"
        ]
        Resource = [
          var.s3_bucket_arn,
          "${var.s3_bucket_arn}/*"
        ]
      }
    ]
  })
}

# ── Instance profile ──────────────────────────────────────────────────────────
resource "aws_iam_instance_profile" "ec2_profile" {
  name = "${var.name_prefix}-ec2-profile"
  role = aws_iam_role.ec2_role.name

  tags = {
    Name        = "${var.name_prefix}-ec2-profile"
    Environment = var.environment
    Project     = "album-store"
  }
}
