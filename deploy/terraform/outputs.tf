output "lambda_function_arn" {
  description = "ARN of the ami-update-automation Lambda function."
  value       = aws_lambda_function.ami_updater.arn
}

output "lambda_function_name" {
  description = "Name of the Lambda function."
  value       = aws_lambda_function.ami_updater.function_name
}

output "lambda_role_arn" {
  description = "ARN of the IAM role attached to the Lambda function."
  value       = aws_iam_role.lambda.arn
}

output "github_secret_arn" {
  description = "ARN of the Secrets Manager secret holding the GitHub token."
  value       = local.github_secret_arn
}

output "schedule_rule_arn" {
  description = "ARN of the EventBridge schedule rule."
  value       = aws_cloudwatch_event_rule.schedule.arn
}
