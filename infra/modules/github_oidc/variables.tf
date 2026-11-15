variable "env" {
  description = "Environment identifier used as a prefix of resource names."
  type        = string
}

variable "github_oidc_sub_prefix" {
  description = "Prefix of the sub claim in GitHub OIDC tokens issued for the repository (e.g. repo:<owner>@<owner_id>/<repo>@<repo_id> when immutable subjects are enabled)."
  type        = string

  validation {
    condition     = startswith(var.github_oidc_sub_prefix, "repo:")
    error_message = "github_oidc_sub_prefix must start with repo:."
  }
}

variable "tfstate_bucket" {
  description = "Name of the S3 bucket that stores the Terraform state."
  type        = string
}

variable "tfstate_key" {
  description = "Object key of the Terraform state in the bucket."
  type        = string
}
