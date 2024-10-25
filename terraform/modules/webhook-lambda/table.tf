module "dynamodb_table" {
  source = "../dynamodb"

  table_name   = "${local.profile}-table"
  billing_mode = "PROVISIONED"

  hash_key  = "login"
  range_key = "repo-env"

  read_capacity  = 5
  write_capacity = 5
}