# Provided via Terragrunt inputs or defaults
variable "table_name" {
  description = "The name of the DynamoDB table."
  type        = string
}

variable "billing_mode" {
  description = "The billing mode for the DynamoDB table."
  type        = string
  default     = "PROVISIONED"
}

variable "hash_key" {
  description = "The name of the attribute to be used as the hash(partition) key."
  type        = string
}

variable "range_key" {
  description = "The name of the attribute to be used as the range(sort) key."
  type        = string
  default     = null
}

variable "read_capacity" {
  description = "The number of read capacity units for the table (required if billing_mode is PROVISIONED)."
  type        = number
  default     = 5
}

variable "write_capacity" {
  description = "The number of write capacity units for the table (required if billing_mode is PROVISIONED)."
  type        = number
  default     = 5
}

variable "global_secondary_indexes" {
  description = "List of global secondary indexes"
  type = list(object({
    name            = string
    hash_key        = string
    range_key       = string
    projection_type = string
    read_capacity   = number
    write_capacity  = number
  }))
  default = []
}
