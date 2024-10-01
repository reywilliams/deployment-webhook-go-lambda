locals {
  profile = "${var.project_name}-${var.environment}"

  gateway_trigger_files = [
    "gateway.tf",
    "integration.tf",
    "deployment.tf",
    "responses.tf",
  ]
  gateway_file_hashes         = { for file in local.gateway_trigger_files : file => md5(file(file)) }
  gateway_files_combined_hash = md5(join("", [for file in local.gateway_trigger_files : local.gateway_file_hashes[file]]))

  POST_METHOD             = "POST"
  USAGE_PLAN_API_KEY_TYPE = "API_KEY"
  QUOTA_PERIOD_DAY        = "DAY"
}