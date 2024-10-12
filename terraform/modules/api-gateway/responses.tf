# to surface a successful response to the client
resource "aws_api_gateway_method_response" "resp_200" {
  rest_api_id = aws_api_gateway_rest_api.webhook.id
  resource_id = aws_api_gateway_resource.webhook.id
  http_method = aws_api_gateway_method.post_webhook.http_method
  status_code = "200"
}

# to surface an unsuccessful responses to the client
resource "aws_api_gateway_method_response" "resp_400" {
  rest_api_id = aws_api_gateway_rest_api.webhook.id
  resource_id = aws_api_gateway_resource.webhook.id
  http_method = aws_api_gateway_method.post_webhook.http_method
  status_code = "400"
}

resource "aws_api_gateway_method_response" "resp_500" {
  rest_api_id = aws_api_gateway_rest_api.webhook.id
  resource_id = aws_api_gateway_resource.webhook.id
  http_method = aws_api_gateway_method.post_webhook.http_method
  status_code = "500"
}