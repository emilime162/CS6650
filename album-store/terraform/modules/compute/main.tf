# ─────────────────────────────────────────────────────────────────────────────
# Compute Module: EC2 instance for album-store
# ─────────────────────────────────────────────────────────────────────────────

# ── Get latest Amazon Linux 2023 AMI ──────────────────────────────────────────
data "aws_ami" "amazon_linux" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-*-x86_64"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# ── EC2 instances ─────────────────────────────────────────────────────────────
resource "aws_instance" "album_store" {
  count                  = var.instance_count
  ami                    = data.aws_ami.amazon_linux.id
  instance_type          = var.instance_type
  key_name               = var.key_name
  iam_instance_profile   = var.iam_instance_profile
  vpc_security_group_ids = [var.security_group_id]

  user_data = templatefile("${path.module}/user_data.sh", {
    aws_region     = var.aws_region
    albums_table   = var.albums_table_name
    photos_table   = var.photos_table_name
    s3_bucket      = var.s3_bucket_name
    go_version     = var.go_version
    git_repo_url   = var.git_repo_url
    git_branch     = var.git_branch
  })

  root_block_device {
    volume_size = var.root_volume_size
    volume_type = "gp3"
  }

  tags = {
    Name        = "${var.name_prefix}-instance-${count.index + 1}"
    Environment = var.environment
    Project     = "album-store"
  }
}

# ── ALB Target Group Attachment ───────────────────────────────────────────────
resource "aws_lb_target_group_attachment" "album_store" {
  count            = var.target_group_arn != "" ? var.instance_count : 0
  target_group_arn = var.target_group_arn
  target_id        = aws_instance.album_store[count.index].id
  port             = 8080
}
