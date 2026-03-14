# ECR repo for Lambda image
resource "aws_ecr_repository" "lambda" {
  name = "cs6650l2-lambda"
}

# Build and push Lambda image — platform=linux/amd64 fixes architecture mismatch on M2 Mac
resource "docker_image" "lambda" {
  name = "${aws_ecr_repository.lambda.repository_url}:latest"
  build {
    context  = "../src/lambda"
    platform = "linux/amd64"    # ← this is the fix!
  }
}

resource "docker_registry_image" "lambda" {
  name = docker_image.lambda.name
}

resource "aws_lambda_function" "order_processor" {
  function_name = "${var.service_name}-order-processor"
  role          = data.aws_iam_role.lab_role.arn
  package_type  = "Image"
  image_uri     = "${aws_ecr_repository.lambda.repository_url}:latest"
  memory_size   = 512
  timeout       = 30
  architectures = ["x86_64"]   # ← explicitly match amd64

  depends_on = [docker_registry_image.lambda]
}

resource "aws_lambda_permission" "sns" {
  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.order_processor.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = module.messaging.sns_topic_arn
}

resource "aws_sns_topic_subscription" "lambda" {
  topic_arn = module.messaging.sns_topic_arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.order_processor.arn
}

resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${var.service_name}-order-processor"
  retention_in_days = 7
}