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

# GitHub の OIDC トークンの sub の前半 (このリポジトリは、ID を含む immutable subject の形式)
# 値は gh api repos/<owner>/<repo>/actions/oidc/customization/sub の sub_claim_prefix で確かめられる
variable "github_oidc_sub_prefix" {
  description = "Prefix of the sub claim in GitHub OIDC tokens issued for this repository."
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

# 時間課金のリソース (ALB と DNS、インターフェース型エンドポイント、ECS) をまとめて作るかどうか
variable "enable_runtime" {
  description = "Whether to create the billed runtime resources: the ALB with its DNS record and certificate, the interface VPC endpoints, and the ECS cluster and service. Set to false while the environment is idle."
  type        = bool
}

variable "api_image_digest" {
  description = "Digest (sha256:...) of the api image in ECR used by the first task definition. Later revisions are registered by the CD workflow."
  type        = string

  validation {
    condition     = can(regex("^sha256:[0-9a-f]{64}$", var.api_image_digest))
    error_message = "api_image_digest must be in the form sha256:<64 hex characters>."
  }
}

variable "zone_name" {
  description = "Name of the existing public hosted zone to create the records in."
  type        = string
}

variable "domain_name" {
  description = "Fully qualified domain name of this project."
  type        = string
}
