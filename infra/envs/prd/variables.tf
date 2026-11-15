variable "env" {
  description = "Environment identifier."
  type        = string

  validation {
    condition     = contains(["dev", "prd"], var.env)
    error_message = "env must be one of dev or prd."
  }
}

variable "region" {
  description = "AWS region to deploy the resources into."
  type        = string
}

variable "vpc_cidr" {
  description = "CIDR block of the VPC."
  type        = string

  validation {
    condition     = can(cidrhost(var.vpc_cidr, 0))
    error_message = "vpc_cidr must be a valid IPv4 CIDR block."
  }
}

variable "enable_interface_endpoints" {
  description = "Whether to create the interface VPC endpoints. Set to false to cut the cost while the environment is idle."
  type        = bool
}

variable "github_repository" {
  description = "GitHub repository allowed to assume the CI roles, in the form of owner/name."
  type        = string
}

# backend.tf の bucket と key は変数を使えないため、同じ値をここでも渡す
variable "tfstate_bucket" {
  description = "Name of the S3 bucket that stores the Terraform state. Must match backend.tf."
  type        = string
}

variable "tfstate_key" {
  description = "Object key of the Terraform state in the bucket. Must match backend.tf."
  type        = string
}
