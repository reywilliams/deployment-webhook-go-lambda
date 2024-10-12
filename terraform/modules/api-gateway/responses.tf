# to surface a successful response to the client
resource "aws_api_gateway_method_response" "resp_200" {
  rest_api_id = aws_api_gateway_rest_api.webhook.id
  resource_id = aws_api_gateway_resource.webhook.id
  http_method = aws_api_gateway_method.post_webhook.http_method
  status_code = "200"
}

# to surface an unsuccessful response to the client
resource "aws_api_gateway_method_response" "resp_400" {
  rest_api_id = aws_api_gateway_rest_api.webhook.id
  resource_id = aws_api_gateway_resource.webhook.id
  http_method = aws_api_gateway_method.post_webhook.http_method
  status_code = "400"
}

# to map a successful lambda function response to the API Gateway’s response
resource "aws_api_gateway_integration_response" "resp_200" {
  rest_api_id = aws_api_gateway_rest_api.webhook.id
  resource_id = aws_api_gateway_resource.webhook.id
  http_method = aws_api_gateway_method.post_webhook.http_method
  status_code = aws_api_gateway_method_response.resp_200.status_code

  # used to transform the integration response body
  # when content type is application/json
  # the templates use Velocity Template Language (VTL) Syntax
  # TODO: remove of update when you have a working integration/backend
  # response_templates = {
  #   "application/json" = jsonencode({
  #     "statusCode" = 200
  #   })
  # }

  depends_on = [
    aws_api_gateway_integration.webhook_lambda
  ]
}

# to map an unsuccessful lambda function’s response to the API Gateway’s response
resource "aws_api_gateway_integration_response" "resp_400" {
  rest_api_id = aws_api_gateway_rest_api.webhook.id
  resource_id = aws_api_gateway_resource.webhook.id
  http_method = aws_api_gateway_method.post_webhook.http_method
  status_code = aws_api_gateway_method_response.resp_400.status_code

  # used to transform the integration response body
  # when content type is application/json
  # response_templates = {
  #   "application/json" = jsonencode({
  #     message = "Error: Bad Request"
  #   })
  # }

  # pattern used to match the backend response
  # in this case, all 4XX status code (per AWS docs)
  selection_pattern = "4\\d{2}"

  depends_on = [
    aws_api_gateway_integration.webhook_lambda
  ]
}