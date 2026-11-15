variable "env" {
  description = "Environment identifier used as a prefix of resource names."
  type        = string
}

variable "vpc_cidr" {
  description = "CIDR block of the VPC. Subnets are carved out of it with cidrsubnet."
  type        = string
}

variable "az_count" {
  description = "Number of availability zones to spread the subnets across."
  type        = number
  default     = 2
}

variable "enable_interface_endpoints" {
  description = "Whether to create the interface VPC endpoints (ecr.api, ecr.dkr, logs), which are billed per hour per AZ."
  type        = bool
  default     = true
}
