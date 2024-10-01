module "dynamodb_table" {
  source = "../dynamodb"

  table_name   = "${local.profile}-deployment-engineers"
  billing_mode = "PROVISIONED"

  hash_key  = "email"
  range_key = "repository"

  read_capacity  = 5
  write_capacity = 5
}