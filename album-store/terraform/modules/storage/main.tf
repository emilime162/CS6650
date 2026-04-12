# ─────────────────────────────────────────────────────────────────────────────
# Storage Module: DynamoDB tables + S3 bucket
# ─────────────────────────────────────────────────────────────────────────────

# ── DynamoDB: albums table ────────────────────────────────────────────────────
resource "aws_dynamodb_table" "albums" {
  name         = var.albums_table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "album_id"

  attribute {
    name = "album_id"
    type = "S"
  }

  tags = {
    Name        = var.albums_table_name
    Environment = var.environment
    Project     = "album-store"
  }
}

# ── DynamoDB: photos table ────────────────────────────────────────────────────
resource "aws_dynamodb_table" "photos" {
  name         = var.photos_table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "album_id"
  range_key    = "photo_id"

  attribute {
    name = "album_id"
    type = "S"
  }

  attribute {
    name = "photo_id"
    type = "S"
  }

  tags = {
    Name        = var.photos_table_name
    Environment = var.environment
    Project     = "album-store"
  }
}

# ── S3 bucket ─────────────────────────────────────────────────────────────────
resource "aws_s3_bucket" "photos" {
  bucket = var.s3_bucket_name

  tags = {
    Name        = var.s3_bucket_name
    Environment = var.environment
    Project     = "album-store"
  }
}

# ── Disable block public access ───────────────────────────────────────────────
resource "aws_s3_bucket_public_access_block" "photos" {
  bucket = aws_s3_bucket.photos.id

  block_public_acls       = false
  block_public_policy     = false
  ignore_public_acls      = false
  restrict_public_buckets = false
}

# ── Bucket policy: public read ────────────────────────────────────────────────
resource "aws_s3_bucket_policy" "photos_public_read" {
  bucket = aws_s3_bucket.photos.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "PublicRead"
        Effect    = "Allow"
        Principal = "*"
        Action    = "s3:GetObject"
        Resource  = "${aws_s3_bucket.photos.arn}/*"
      }
    ]
  })

  depends_on = [aws_s3_bucket_public_access_block.photos]
}
