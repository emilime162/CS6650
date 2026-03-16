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

# ALB in public subnets
module "alb" {
  source                = "./modules/alb"
  service_name          = var.service_name
  subnet_ids            = module.network.public_subnet_ids
  vpc_id                = module.network.vpc_id
  container_port        = var.container_port
  alb_security_group_id = module.network.alb_security_group_id
}

module "messaging" {
  source       = "./modules/messaging"
  service_name = var.service_name
}

# Receiver — /sync and /async, behind ALB, in private subnets
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
    { name = "SERVICE_MODE",  value = "receiver" },
    { name = "SNS_TOPIC_ARN", value = module.messaging.sns_topic_arn },
  ]
}

# Processor — SQS workers, no ALB, in private subnets
resource "aws_ecs_task_definition" "processor" {
  family                   = "${var.service_name}-processor"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([{
    name  = "processor"
    image = "${module.ecr.repository_url}:latest"
    environment = [
      { name = "SERVICE_MODE",  value = "processor" },
      { name = "SQS_QUEUE_URL", value = module.messaging.sqs_queue_url },
      { name = "NUM_WORKERS",   value = tostring(var.num_workers) },
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = "/ecs/${var.service_name}-processor"
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "ecs"
      }
    }
  }])
}

resource "aws_cloudwatch_log_group" "processor" {
  name              = "/ecs/${var.service_name}-processor"
  retention_in_days = var.log_retention_days
}

resource "aws_ecs_service" "processor" {
  name            = "${var.service_name}-processor"
  cluster         = module.ecs.cluster_name
  task_definition = aws_ecs_task_definition.processor.arn
  desired_count   = 1
  launch_type     = "FARGATE"


  network_configuration {
    subnets         = module.network.private_subnet_ids
    security_groups = [module.network.security_group_id]
  }
}

resource "docker_image" "app" {
  name = "${module.ecr.repository_url}:latest"
  build {
    context = "../src"
  }
}

resource "docker_registry_image" "app" {
  name = docker_image.app.name
}