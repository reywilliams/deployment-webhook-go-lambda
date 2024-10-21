module "github_PAT_secret" {
  source = "../secret"

  secret_name        = var.github_PAT_secret_name
  secret_string      = var.github_PAT_secret_string
  secret_description = "The secret for the GitHub PAT, used to approve requests."
}

module "github_webhook_secret" {
  source = "../secret"

  secret_name        = var.github_webhook_secret_name
  secret_string      = var.github_webhook_secret_string
  secret_description = "The secret for the GitHub webhooks, used to verify payloads."
}

