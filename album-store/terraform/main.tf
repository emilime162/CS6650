# ─────────────────────────────────────────────────────────────────────────────
# Album Store Infrastructure with Load Balancing
# ─────────────────────────────────────────────────────────────────────────────

# ── Storage Module ────────────────────────────────────────────────────────────
module "storage" {
  source = "./modules/storage"

  albums_table_name = var.albums_table_name
  photos_table_name = var.photos_table_name
  s3_bucket_name    = var.s3_bucket_name
  environment       = var.environment
}

# ── IAM Module ────────────────────────────────────────────────────────────────
module "iam" {
  source = "./modules/iam"

  name_prefix       = var.name_prefix
  environment       = var.environment
  albums_table_arn  = module.storage.albums_table_arn
  photos_table_arn  = module.storage.photos_table_arn
  s3_bucket_arn     = module.storage.s3_bucket_arn
}

# ── ALB Module ────────────────────────────────────────────────────────────────
module "alb" {
  source = "./modules/alb"

  name_prefix = var.name_prefix
  environment = var.environment
  vpc_id      = module.networking.vpc_id
}

# ── Networking Module ─────────────────────────────────────────────────────────
module "networking" {
  source = "./modules/networking"

  name_prefix            = var.name_prefix
  environment            = var.environment
  allowed_cidr_blocks    = var.allowed_cidr_blocks
  ssh_cidr_blocks        = var.ssh_cidr_blocks
  alb_security_group_id  = module.alb.alb_security_group_id
}

# ── Compute Module ────────────────────────────────────────────────────────────
module "compute" {
  source = "./modules/compute"

  name_prefix          = var.name_prefix
  environment          = var.environment
  instance_type        = var.instance_type
  instance_count       = var.instance_count
  key_name             = var.key_name
  iam_instance_profile = module.iam.instance_profile_name
  security_group_id    = module.networking.security_group_id
  target_group_arn     = module.alb.target_group_arn
  aws_region           = var.aws_region
  albums_table_name    = module.storage.albums_table_name
  photos_table_name    = module.storage.photos_table_name
  s3_bucket_name       = module.storage.s3_bucket_name
  go_version           = var.go_version
  git_repo_url         = var.git_repo_url
  git_branch           = var.git_branch
}
