# AWS assume role policy, this allows the lambda to 
# assume the role created here
data "aws_iam_policy_document" "assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }

    actions = ["sts:AssumeRole"]
  }
}

# data source for the AWSLambdaBasicExecutionRole managed policy
# allows lambda to CREATE log groups and streams and also PUT/write log events
# https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AWSLambdaBasicExecutionRole.html
data "aws_iam_policy" "lambda_basic_execution" {
  arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# allows lambda to get items
# specifically scoped to the table from the dynamodb_table module
resource "aws_iam_policy" "lambda_dynamodb_write_policy" {
  name        = "lambda_dynamodb_policy"
  description = "Policy to allow Lambda functions to fetch from DynamoDB"

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect = "Allow",
        Action = [
          # "dynamodb:PutItem",
          "dynamodb:GetItem",
          # "dynamodb:UpdateItem",
          # "dynamodb:BatchWriteItem"
        ],
        Resource = module.dynamodb_table.table_arn
      }
    ]
  })
}

data "aws_iam_policy" "xray" {
  arn = "arn:aws:iam::aws:policy/AWSXRayDaemonWriteAccess"
}

# allows lambda to access the github PAT and webhook secrets
# using their ARNs
locals {
  secret_arns = [module.github_webhook_secret.secret_ARN, module.github_PAT_secret.secret_ARN]
}
resource "aws_iam_policy" "secret_access" {
  name = "secrets-access-policy"

  policy = <<EOF
  {
    "Version": "2012-10-17",
    "Statement": [
      {
        "Action": [
          "secretsmanager:*"
        ],
        "Effect": "Allow",
        "Resource": ${jsonencode(local.secret_arns)}
      }
    ]
  }
  EOF
}

# Define the IAM role for Lambda execution
resource "aws_iam_role" "lambda_execution" {
  name = "${local.profile}-lambda-execution-role"

  assume_role_policy = data.aws_iam_policy_document.assume_role.json

  # ensures this policies are always attached, if removed will be reattched
  # if any added outside tf state, will be removed
  managed_policy_arns = [data.aws_iam_policy.lambda_basic_execution.arn, aws_iam_policy.lambda_dynamodb_write_policy.arn,
  data.aws_iam_policy.xray.arn, aws_iam_policy.secret_access.arn]
}
