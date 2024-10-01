resource "aws_dynamodb_table" "table" {
  name           = var.table_name
  billing_mode   = var.billing_mode
  hash_key       = var.hash_key
  range_key      = var.range_key
  read_capacity  = var.read_capacity
  write_capacity = var.write_capacity

  # dynamically create GSIs if they are passed in
  dynamic "global_secondary_index" {
    for_each = var.global_secondary_indexes
    content {
      name            = global_secondary_index.value.name
      hash_key        = global_secondary_index.value.hash_key
      range_key       = global_secondary_index.value.range_key
      projection_type = global_secondary_index.value.projection_type
      read_capacity   = global_secondary_index.value.read_capacity
      write_capacity  = global_secondary_index.value.write_capacity
    }
  }

  attribute {
    name = var.hash_key
    type = "S" # assumption, TODO: challenge if needed
  }

  # only create attribute block for range_key if
  # range_key is provided 
  dynamic "attribute" {
    for_each = var.range_key != null ? [var.range_key] : []
    content {
      name = var.range_key
      type = "S" # assumption, TODO: challenge if needed
    }
  }

}