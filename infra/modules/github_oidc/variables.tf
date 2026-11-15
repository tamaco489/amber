variable "env" {
  description = "Environment identifier used as a prefix of resource names."
  type        = string
}

variable "github_repository" {
  description = "GitHub repository allowed to assume the roles, in the form of owner/name."
  type        = string
}

variable "tfstate_bucket" {
  description = "Name of the S3 bucket that stores the Terraform state."
  type        = string
}

variable "tfstate_key" {
  description = "Object key of the Terraform state in the bucket."
  type        = string
}
