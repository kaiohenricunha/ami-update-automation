locals {
  github_secret_arn = var.github_token_secret_arn != "" ? var.github_token_secret_arn : aws_secretsmanager_secret.github_token[0].arn
}

# Optional: create a new Secrets Manager secret if no existing ARN is provided.
resource "aws_secretsmanager_secret" "github_token" {
  count       = var.github_token_secret_arn == "" ? 1 : 0
  name        = "${var.function_name}/github-token"
  kms_key_id  = var.kms_key_arn != "" ? var.kms_key_arn : null
  tags        = var.tags
}

resource "aws_secretsmanager_secret_version" "github_token" {
  count         = var.github_token_secret_arn == "" ? 1 : 0
  secret_id     = aws_secretsmanager_secret.github_token[0].id
  secret_string = var.github_token
}

# CloudWatch Log Group.
resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${var.function_name}"
  retention_in_days = var.log_retention_days
  kms_key_id        = var.kms_key_arn != "" ? var.kms_key_arn : null
  tags              = var.tags
}

# Lambda Function.
resource "aws_lambda_function" "ami_updater" {
  function_name = var.function_name
  role          = aws_iam_role.lambda.arn
  filename      = var.lambda_zip_path
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = [var.lambda_architecture]
  timeout       = var.lambda_timeout
  memory_size   = var.lambda_memory_size

  source_code_hash = filebase64sha256(var.lambda_zip_path)

  environment {
    variables = {
      CONFIG_PATH = "/var/task/config.yaml"
      LOG_LEVEL   = var.log_level
    }
  }

  dynamic "vpc_config" {
    for_each = var.vpc_config != null ? [var.vpc_config] : []
    content {
      subnet_ids         = vpc_config.value.subnet_ids
      security_group_ids = vpc_config.value.security_group_ids
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.lambda,
    aws_iam_role_policy_attachment.basic,
  ]

  tags = var.tags
}

# EventBridge rule to trigger Lambda on schedule.
resource "aws_cloudwatch_event_rule" "schedule" {
  name                = "${var.function_name}-schedule"
  schedule_expression = var.schedule_expression
  tags                = var.tags
}

resource "aws_cloudwatch_event_target" "lambda" {
  rule      = aws_cloudwatch_event_rule.schedule.name
  target_id = var.function_name
  arn       = aws_lambda_function.ami_updater.arn
}

resource "aws_lambda_permission" "eventbridge" {
  statement_id  = "AllowEventBridgeInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.ami_updater.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.schedule.arn
}
