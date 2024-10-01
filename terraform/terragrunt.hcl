# ---------------------------------------------------------------------------------------------------------------------
# TERRAGRUNT CONFIGURATION
# ---------------------------------------------------------------------------------------------------------------------

locals {
  # Automatically load common variables
  common_vars = read_terragrunt_config(find_in_parent_folders("common.hcl"))

  # Automatically load account-level variables
  account_vars = read_terragrunt_config(find_in_parent_folders("account.hcl"))

  # Automatically load region-level variables
  region_vars = read_terragrunt_config(find_in_parent_folders("region.hcl"))

  # Automatically load environment-level variables
  environment_vars = read_terragrunt_config(find_in_parent_folders("environment.hcl"))

  # Extract the variables we need for easy access
  aws_account_id = local.account_vars.locals.aws_account_id
  aws_region     = local.region_vars.locals.aws_region
  environment    = local.environment_vars.locals.environment
  project_name   = local.common_vars.locals.project_name
  profile        = "${local.project_name}-${local.environment}"
}

# Generate an AWS provider block
generate "provider" {
  path      = "provider.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
provider "aws" {
  region  = "${local.aws_region}"
  profile = "${local.profile}"

  # Only these AWS Account IDs may be operated on by this template
  allowed_account_ids = ["${local.aws_account_id}"]

  # default tags
  default_tags {
    tags = {
      ManagedByTerraform = true
      environment = "${local.environment}"
    }
  }
}
EOF
}

# Configure Terragrunt to automatically store tfstate files in an S3 bucket
remote_state {
  backend = "s3"
  config = {
    encrypt        = true
    bucket         = join("-", [local.profile, "terraform", "state"])
    dynamodb_table = join("-", [local.profile, "terraform", "state", "locks"])
    key            = "${path_relative_to_include()}/tf.tfstate"
    region         = local.aws_region
    profile        = local.profile
  }

  generate = {
    path      = "backend.tf"
    if_exists = "overwrite_terragrunt"
  }
}

# ---------------------------------------------------------------------------------------------------------------------
# GLOBAL PARAMETERS
# These variables apply to all configurations in this subfolder. These are automatically merged into the child
# `terragrunt.hcl` config via the include block.
# ---------------------------------------------------------------------------------------------------------------------

# Configure root level variables that all resources can inherit. This is especially helpful with multi-account configs
# where terraform_remote_state data sources are placed directly into the modules.
inputs = merge(
  local.account_vars.locals,
  local.region_vars.locals,
  local.environment_vars.locals,
  local.common_vars.locals
)
