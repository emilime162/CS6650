module "network" {
  source         = "./modules/network"
  service_name   = var.service_name
  container_port = var.container_port
  aws_region     = var.aws_region
}

module "ecr" {
  source          = "./modules/ecr"
  repository_name = var.ecr_repository_name
}

module "logging" {
  source            = "./modules/logging"
  service_name      = var.service_name
  retention_in_days = var.log_retention_days
}

data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

module "alb" {
  source                = "./modules/alb"
  service_name          = var.service_name
  subnet_ids            = module.network.public_subnet_ids
  vpc_id                = module.network.vpc_id
  container_port        = var.container_port
  alb_security_group_id = module.network.alb_security_group_id
}

module "ecs" {
  source             = "./modules/ecs"
  service_name       = var.service_name
  image              = "${module.ecr.repository_url}:latest"
  container_port     = var.container_port
  subnet_ids         = module.network.private_subnet_ids
  security_group_ids = [module.network.security_group_id]
  execution_role_arn = data.aws_iam_role.lab_role.arn
  task_role_arn      = data.aws_iam_role.lab_role.arn
  log_group_name     = module.logging.log_group_name
  ecs_count          = var.ecs_count
  region             = var.aws_region
  target_group_arn   = module.alb.target_group_arn
  alb_listener_arn   = module.alb.alb_listener_arn
  environment = [
    { name = "MYSQL_ENDPOINT",      value = module.rds.endpoint },
    { name = "MYSQL_DB",            value = module.rds.db_name },
    { name = "MYSQL_USER",          value = module.rds.db_username },
    { name = "MYSQL_PASSWORD",      value = var.db_password },
    { name = "DYNAMODB_TABLE",      value = module.dynamodb.table_name },
    { name = "AWS_REGION",          value = var.aws_region },
  ]
}

module "rds" {
  source                = "./modules/rds"
  service_name          = var.service_name
  vpc_id                = module.network.vpc_id
  private_subnet_ids    = module.network.private_subnet_ids
  ecs_security_group_id = module.network.security_group_id
  db_password           = var.db_password
}

module "dynamodb" {
  source       = "./modules/dynamodb"
  service_name = var.service_name
}

resource "docker_image" "app" {
  name = "${module.ecr.repository_url}:latest"
  build {
    context  = "../src"
    platform = "linux/amd64"
  }
}

resource "docker_registry_image" "app" {
  name = docker_image.app.name
}