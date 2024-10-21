# Provided via Terragrunt inputs or defaults

variable "secret_name" {
  description = "name of the secret"
  type        = string
}

variable "secret_description" {
  description = "description of the secret"
  type        = string
  default     = ""
}

variable "json_encode" {
  type        = bool
  description = "flag to mark if secret string should be json encoded"
  default     = false
}

variable "secret_string" {
  description = "secret string/secret value"
  type        = string
  sensitive   = true
}