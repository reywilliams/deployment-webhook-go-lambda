include "root" {
  path = find_in_parent_folders()
}

terraform {
  source = "${get_terragrunt_dir()}/../../../../../modules//api-gateway"
}

dependency "webhook-lambda" {
  config_path = "../webhook-lambda"
}

inputs = {
  aws_lambda_webhook_function_invoke_arn = dependency.webhook-lambda.outputs.invoke_arn
  aws_lambda_webhook_function_name       = dependency.webhook-lambda.outputs.function_name
}