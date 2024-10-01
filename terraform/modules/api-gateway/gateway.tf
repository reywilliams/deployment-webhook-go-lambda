resource "aws_api_gateway_rest_api" "webhook" {
  name        = "${local.profile}-wehbook-rest-api"
  description = "REST API for GitHub webhooks."
}

# base API path
resource "aws_api_gateway_resource" "webhook" {
  parent_id   = aws_api_gateway_rest_api.webhook.root_resource_id
  rest_api_id = aws_api_gateway_rest_api.webhook.id
  path_part   = "webhook"
}