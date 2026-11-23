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

variable "ecr_repository_arn" {
  description = "ARN of the ECR repository that the deploy role pushes the api image to."
  type        = string
}

variable "deploy_event_source" {
  description = "Source of the events that the deploy role may send to the default event bus when it starts a deployment."
  type        = string
}

variable "deploy_event_detail_type" {
  description = "Detail type of the events that the deploy role may send to the default event bus when it starts a deployment."
  type        = string
}

variable "notifier_function_arn" {
  description = "ARN of the notification Lambda function whose code the notifier deploy role may update."
  type        = string
}
