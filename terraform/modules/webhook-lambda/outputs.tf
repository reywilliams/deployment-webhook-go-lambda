output "invoke_arn" {
  value = aws_lambda_function.webhook.invoke_arn
}

output "function_name" {
  value = aws_lambda_function.webhook.function_name
}

output "dynamodb_table_name" {
  value = module.dynamodb_table.table_name
}

output "dynamodb_table_arn" {
  value = module.dynamodb_table.table_arn
}