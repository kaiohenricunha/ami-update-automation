module "ami_updater" {
  source = "git::https://github.com/kaiohenricunha/ami-update-automation.git//deploy/terraform?ref=v1.0.0"

  function_name        = "ami-update-automation"
  schedule_expression  = "rate(6 hours)"
  lambda_architecture  = "arm64"
  lambda_zip_path      = "./lambda-arm64.zip"
  config_yaml_path     = "./config.yaml"

  # Provide either an existing secret ARN or a new token value.
  # Option A: use an existing Secrets Manager secret:
  # github_token_secret_arn = "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-token"
  # Option B: create a new secret:
  github_token = var.github_token

  log_level          = "info"
  log_retention_days = 30

  tags = {
    Team        = "platform"
    Environment = "prod"
    ManagedBy   = "terraform"
  }
}

variable "github_token" {
  description = "GitHub personal access token with repo and pull_request permissions."
  type        = string
  sensitive   = true
}

output "lambda_arn" {
  value = module.ami_updater.lambda_function_arn
}
