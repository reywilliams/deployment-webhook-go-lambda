# Provided via Terragrunt root config
variable "project_name" {
  type        = string
  description = "Name of app, such as reys-cool-project"
}

variable "environment" {
  type        = string
  description = "environment such as prod, dev, test"
}

variable "aws_region" {
  type        = string
  description = "The AWS region being targeted (ex. us-west-2)"
}

variable "aws_account_id" {
  type        = string
  description = "The AWS account ID that is being deployed against. Used to configure allowed_account_ids."
}

# Provided via Terragrunt inputs or defaults
variable "aws_lambda_webhook_function_invoke_arn" {
  type        = string
  description = "Invoke ARN of the webhook lambda."
}

variable "aws_lambda_webhook_function_name" {
  type        = string
  description = "Function name of webhook lambda."
}

# TODO/NOTE: you will have to bump this 
# as a webhook will likely surpass this limit
variable "gateway_usage_quota_limit" {
  type        = number
  description = "plan usage quota limit."
  default     = 50
}