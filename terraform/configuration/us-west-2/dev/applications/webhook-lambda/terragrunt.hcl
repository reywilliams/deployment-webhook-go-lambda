include "root" {
  path = find_in_parent_folders()
}

terraform {
  source = "${get_terragrunt_dir()}/../../../../../..//terraform/modules/webhook-lambda"
}

inputs = {}