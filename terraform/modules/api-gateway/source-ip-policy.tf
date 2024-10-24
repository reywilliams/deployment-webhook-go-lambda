
data "external" "github_webhook_ips" {
  program = ["bash", "-c", "curl -s https://api.github.com/meta | jq -r '.hooks | to_entries | map({(.key | tostring): .value}) | add'"]
}

# restricts API Gateway to source IPs from Github 
# specifically the ones used for hooks 
# see http://api.github.com/meta and look at "hooks"
# and allows GitHub to invoke API with their non-AWS identity
data "aws_iam_policy_document" "only_github_hook_ips_policy" {
  statement {
    effect    = "Deny"
    actions   = ["execute-api:Invoke"]
    resources = ["${aws_api_gateway_rest_api.webhook.execution_arn}/*"]


    condition {
      test     = "NotIpAddress"
      variable = "aws:SourceIp"
      values   = [for ip in data.external.github_webhook_ips.result : ip]
    }

    principals {
      type        = "*"
      identifiers = ["*"]
    }
  }

  statement {
    effect    = "Allow"
    actions   = ["execute-api:Invoke"]
    resources = ["${aws_api_gateway_rest_api.webhook.execution_arn}/*"]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
  }
}


resource "aws_api_gateway_rest_api_policy" "only_github_hook_ips" {
  rest_api_id = aws_api_gateway_rest_api.webhook.id

  policy = data.aws_iam_policy_document.only_github_hook_ips_policy.json
}
