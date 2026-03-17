variable "function_name" {
  description = "Name of the Lambda function."
  type        = string
  default     = "ami-update-automation"
}

variable "schedule_expression" {
  description = "EventBridge schedule expression for the Lambda trigger."
  type        = string
  default     = "rate(6 hours)"
}

variable "lambda_timeout" {
  description = "Lambda function timeout in seconds."
  type        = number
  default     = 300
}

variable "lambda_memory_size" {
  description = "Lambda function memory size in MB."
  type        = number
  default     = 256
}

variable "lambda_architecture" {
  description = "Lambda CPU architecture: arm64 or x86_64."
  type        = string
  default     = "arm64"
  validation {
    condition     = contains(["arm64", "x86_64"], var.lambda_architecture)
    error_message = "lambda_architecture must be arm64 or x86_64."
  }
}

variable "lambda_zip_path" {
  description = "Path to the Lambda deployment zip file."
  type        = string
}

variable "config_yaml_path" {
  description = "Path to the config.yaml file to embed in the Lambda package."
  type        = string
}

variable "github_token_secret_arn" {
  description = "ARN of an existing Secrets Manager secret holding the GitHub token. If empty, a new secret is created."
  type        = string
  default     = ""
}

variable "github_token" {
  description = "GitHub token value. Used only if github_token_secret_arn is empty to create a new secret."
  type        = string
  default     = ""
  sensitive   = true
}

variable "kms_key_arn" {
  description = "Optional KMS key ARN for encrypting the Secrets Manager secret and Lambda environment variables."
  type        = string
  default     = ""
}

variable "vpc_config" {
  description = "Optional VPC configuration for the Lambda function."
  type = object({
    subnet_ids         = list(string)
    security_group_ids = list(string)
  })
  default = null
}

variable "log_retention_days" {
  description = "CloudWatch log group retention period in days."
  type        = number
  default     = 30
}

variable "tags" {
  description = "Tags to apply to all resources."
  type        = map(string)
  default     = {}
}

variable "log_level" {
  description = "Log level for the Lambda function (debug, info, warn, error)."
  type        = string
  default     = "info"
}
