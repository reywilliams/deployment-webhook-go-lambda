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

# check supported runtimes in AWS docs here:
# https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtimes.html#runtimes-supported
variable "lambda_runtime" {
  type        = string
  description = <<EOF
  Runtime for the lambda, i.e java21, python3.1, or 
  provided.al2023 (OS-only runtime) for languages like
  Rust or Go. 
  EOF
  default     = "provided.al2023"
}

variable "github_webhook_secret" {
  type        = string
  description = "Secret value for github webhooks"
  sensitive   = true
}

# TODO: try looking into github app and using
# https://github.com/jferrl/go-githubauth
variable "github_PAT" {
  type        = string
  description = "GitHub Personal Access Token (PAT)"
  sensitive   = true
}