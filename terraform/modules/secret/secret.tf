locals {
  secret_string = var.json_encode ? jsonencode(var.secret_string) : var.secret_string
}

resource "aws_secretsmanager_secret" "this" {
  name        = var.secret_name
  description = var.secret_description
}

resource "aws_secretsmanager_secret_version" "this" {
  secret_id     = aws_secretsmanager_secret.this.id
  secret_string = local.secret_string
}