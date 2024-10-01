locals {
  profile = "${var.project_name}-${var.environment}"

  zipped_lambda_file_path = "${path.module}/../../../lambdas/webhook/archive/lambda.zip"
}