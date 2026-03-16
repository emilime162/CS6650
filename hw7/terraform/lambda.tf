resource "aws_lambda_function" "order_processor" {
  function_name = "${var.service_name}-order-processor"
  role          = data.aws_iam_role.lab_role.arn
  package_type  = "Zip"
  filename      = "${path.module}/../src/lambda/function.zip"
  handler       = "bootstrap"
  runtime       = "provided.al2"
  memory_size   = 512
  timeout       = 30
  architectures = ["x86_64"]
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