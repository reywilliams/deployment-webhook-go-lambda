output "secret_name" {
  value = var.secret_name
}

output "secret_ARN" {
  value = aws_secretsmanager_secret.this.arn
}