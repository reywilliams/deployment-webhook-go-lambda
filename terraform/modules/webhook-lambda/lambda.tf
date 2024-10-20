resource "aws_lambda_function" "webhook" {
  filename      = local.zipped_lambda_file_path
  function_name = "${local.profile}-webhook-lambda"

  # used to trigger replacement when source code
  # of the lambda changes
  source_code_hash = filesha256(local.zipped_lambda_file_path)

  # check supported runtimes in AWS docs here:
  # https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtimes.html#runtimes-supported
  runtime       = var.lambda_runtime
  handler       = "bootstrap"
  architectures = ["arm64"] # as we build lambda with GOARCH=arm64

  role = aws_iam_role.lambda_execution.arn

  environment {
    variables = {
      DYNAMO_DB_TABLE_NAME  = module.dynamodb_table.table_name
      GITHUB_WEBHOOK_SECRET = var.github_webhook_secret
      GITHUB_PAT            = var.github_PAT
    }
  }
}