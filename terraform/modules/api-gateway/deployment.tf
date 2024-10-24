resource "aws_api_gateway_stage" "env_stage" {
  deployment_id = aws_api_gateway_deployment.webhook.id
  rest_api_id   = aws_api_gateway_rest_api.webhook.id
  stage_name    = var.environment
}

resource "aws_api_gateway_deployment" "webhook" {
  rest_api_id = aws_api_gateway_rest_api.webhook.id
  description = "${local.profile} deployment for ${aws_api_gateway_rest_api.webhook.name} ${aws_api_gateway_method.post_webhook.http_method} endpoint."

  # trigger a new deployment if any file in gateway_trigger_files[] is changed
  triggers = {
    redeploy = local.gateway_files_combined_hash
  }

  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    aws_api_gateway_integration.webhook_lambda
  ]
}

# The usage plan and the associated API key allow for the use of the API in a particular stage
resource "aws_api_gateway_usage_plan" "env_stage" {
  name        = "${local.profile}-usage-plan-${aws_api_gateway_method.post_webhook.http_method}-method"
  description = "${local.profile} usage plan for REST API ${aws_api_gateway_rest_api.webhook.name} ${aws_api_gateway_method.post_webhook.http_method} method - '${aws_api_gateway_stage.env_stage.stage_name}' stage"

  quota_settings {
    limit  = var.gateway_usage_quota_limit
    period = local.QUOTA_PERIOD_DAY
  }

  api_stages {
    api_id = aws_api_gateway_rest_api.webhook.id
    stage  = aws_api_gateway_stage.env_stage.stage_name
  }
}

resource "aws_api_gateway_api_key" "webhook" {
  count       = var.use_api_key ? 1 : 0
  name        = "${local.profile}-api-key-${aws_api_gateway_method.post_webhook.http_method}-${aws_api_gateway_resource.webhook.path_part}"
  description = "${local.profile} API key for REST API ${aws_api_gateway_rest_api.webhook.name} ${aws_api_gateway_method.post_webhook.http_method} method - '${aws_api_gateway_stage.env_stage.stage_name}' stage"
}

resource "aws_api_gateway_usage_plan_key" "env_stage" {
  count         = var.use_api_key ? 1 : 0
  key_id        = aws_api_gateway_api_key.webhook.id
  key_type      = local.USAGE_PLAN_API_KEY_TYPE
  usage_plan_id = aws_api_gateway_usage_plan.env_stage.id
}
