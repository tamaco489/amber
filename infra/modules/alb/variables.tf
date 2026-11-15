variable "env" {
  description = "Environment identifier used as a prefix of resource names."
  type        = string
}

variable "vpc_id" {
  description = "ID of the VPC to create the ALB and target groups in."
  type        = string
}

variable "vpc_cidr" {
  description = "CIDR block of the VPC. The ALB sends traffic to targets only within it."
  type        = string
}

variable "public_subnet_ids" {
  description = "IDs of the public subnets to place the ALB in (at least two AZs)."
  type        = list(string)
}

variable "certificate_arn" {
  description = "ARN of the issued ACM certificate for the HTTPS listeners."
  type        = string
}

variable "target_port" {
  description = "Port that the application listens on in the tasks."
  type        = number
  default     = 8080
}

variable "health_check_path" {
  description = "Path that the target groups use for the health check."
  type        = string
  default     = "/health"
}

variable "deregistration_delay" {
  description = "Seconds to wait for in-flight requests before a deregistering target is removed. Keep it longer than the shutdown timeout of the application."
  type        = number
  default     = 30
}
