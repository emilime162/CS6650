# ─────────────────────────────────────────────────────────────────────────────
# Networking Module: Security group for album-store
# ─────────────────────────────────────────────────────────────────────────────

# ── Default VPC ───────────────────────────────────────────────────────────────
data "aws_vpc" "default" {
  default = true
}

# ── Security group ────────────────────────────────────────────────────────────
resource "aws_security_group" "album_store" {
  name        = "${var.name_prefix}-sg"
  description = "Security group for album-store EC2 instance"
  vpc_id      = data.aws_vpc.default.id

  # Port 22: SSH
  ingress {
    description = "SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = var.ssh_cidr_blocks
  }

  # Allow all outbound traffic
  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name        = "${var.name_prefix}-sg"
    Environment = var.environment
    Project     = "album-store"
  }
}

# ── Security group rule: Allow 8080 from ALB (if ALB exists) ─────────────────
resource "aws_security_group_rule" "from_alb" {
  count                    = var.alb_security_group_id != "" ? 1 : 0
  type                     = "ingress"
  from_port                = 8080
  to_port                  = 8080
  protocol                 = "tcp"
  source_security_group_id = var.alb_security_group_id
  security_group_id        = aws_security_group.album_store.id
  description              = "Allow 8080 from ALB"
}

# ── Security group rule: Allow 8080 from anywhere (if no ALB) ────────────────
resource "aws_security_group_rule" "from_internet" {
  count             = var.alb_security_group_id == "" ? 1 : 0
  type              = "ingress"
  from_port         = 8080
  to_port           = 8080
  protocol          = "tcp"
  cidr_blocks       = var.allowed_cidr_blocks
  security_group_id = aws_security_group.album_store.id
  description       = "Allow 8080 from internet"
}
