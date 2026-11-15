variable "zone_name" {
  description = "Name of the existing public hosted zone (managed by another project) to create the records in."
  type        = string
}

variable "domain_name" {
  description = "Fully qualified domain name of this project. A dedicated ACM certificate is issued for it."
  type        = string

  validation {
    condition     = endswith(var.domain_name, ".${var.zone_name}")
    error_message = "domain_name must be a subdomain of zone_name."
  }
}

variable "alb_dns_name" {
  description = "DNS name of the ALB that the domain points to."
  type        = string
}

variable "alb_zone_id" {
  description = "Hosted zone ID of the ALB, used for the alias record."
  type        = string
}
