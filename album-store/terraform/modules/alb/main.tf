# ─────────────────────────────────────────────────────────────────────────────
# ALB Module: Application Load Balancer for album-store
# ─────────────────────────────────────────────────────────────────────────────

# ── Get default VPC subnets ───────────────────────────────────────────────────
data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [var.vpc_id]
  }
}

# ── ALB Security Group ────────────────────────────────────────────────────────
resource "aws_security_group" "alb" {
  name        = "${var.name_prefix}-alb-sg"
  description = "Security group for album-store ALB"
  vpc_id      = var.vpc_id

  # Allow HTTP from anywhere
  ingress {
    description = "HTTP from anywhere"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Allow all outbound
  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name        = "${var.name_prefix}-alb-sg"
    Environment = var.environment
    Project     = "album-store"
  }
}

# ── Application Load Balancer ─────────────────────────────────────────────────
resource "aws_lb" "album_store" {
  name               = "${var.name_prefix}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = data.aws_subnets.default.ids

  enable_deletion_protection = false
  enable_http2               = true

  tags = {
    Name        = "${var.name_prefix}-alb"
    Environment = var.environment
    Project     = "album-store"
  }
}

# ── Target Group ──────────────────────────────────────────────────────────────
resource "aws_lb_target_group" "album_store" {
  name     = "${var.name_prefix}-tg"
  port     = 8080
  protocol = "HTTP"
  vpc_id   = var.vpc_id

  health_check {
    enabled             = true
    path                = "/health"
    port                = "8080"
    protocol            = "HTTP"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 10
    matcher             = "200"
  }

  deregistration_delay = 30

  tags = {
    Name        = "${var.name_prefix}-tg"
    Environment = var.environment
    Project     = "album-store"
  }
}

# ── ALB Listener ──────────────────────────────────────────────────────────────
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.album_store.arn
  port              = "80"
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.album_store.arn
  }
}
