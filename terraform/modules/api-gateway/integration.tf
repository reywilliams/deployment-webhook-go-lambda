resource "aws_api_gateway_method" "post_webhook" {
  rest_api_id      = aws_api_gateway_rest_api.webhook.id
  resource_id      = aws_api_gateway_resource.webhook.id
  api_key_required = var.use_api_key

  http_method   = local.POST_METHOD
  authorization = "NONE" 
}


resource "aws_api_gateway_integration" "webhook_lambda" {
  resource_id = aws_api_gateway_resource.webhook.id
  rest_api_id = aws_api_gateway_rest_api.webhook.id

  # former is how clients will interact with the API
  # latter is how the API will iteract with backend services
  http_method             = aws_api_gateway_method.post_webhook.http_method
  integration_http_method = aws_api_gateway_method.post_webhook.http_method

  type = "AWS_PROXY"
  uri  = var.aws_lambda_webhook_function_invoke_arn
}

resource "aws_lambda_permission" "api_gateway_permission" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = var.aws_lambda_webhook_function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.webhook.execution_arn}/*"
}
