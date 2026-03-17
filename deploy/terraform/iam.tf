data "aws_iam_policy_document" "assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "lambda" {
  name               = "${var.function_name}-role"
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
  tags               = var.tags
}

resource "aws_iam_role_policy_attachment" "basic" {
  role       = aws_iam_role.lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy_attachment" "vpc" {
  count      = var.vpc_config != null ? 1 : 0
  role       = aws_iam_role.lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"
}

data "aws_iam_policy_document" "lambda_permissions" {
  statement {
    sid    = "SSMReadAMIParameters"
    effect = "Allow"
    actions = ["ssm:GetParameter"]
    resources = [
      "arn:aws:ssm:*:*:parameter/aws/service/eks/optimized-ami/*"
    ]
  }

  statement {
    sid    = "SecretsManagerGetToken"
    effect = "Allow"
    actions = ["secretsmanager:GetSecretValue"]
    resources = [local.github_secret_arn]
  }

  dynamic "statement" {
    for_each = var.kms_key_arn != "" ? [1] : []
    content {
      sid    = "KMSDecrypt"
      effect = "Allow"
      actions = [
        "kms:Decrypt",
        "kms:DescribeKey",
      ]
      resources = [var.kms_key_arn]
    }
  }
}

resource "aws_iam_policy" "lambda_permissions" {
  name   = "${var.function_name}-permissions"
  policy = data.aws_iam_policy_document.lambda_permissions.json
  tags   = var.tags
}

resource "aws_iam_role_policy_attachment" "lambda_permissions" {
  role       = aws_iam_role.lambda.name
  policy_arn = aws_iam_policy.lambda_permissions.arn
}
