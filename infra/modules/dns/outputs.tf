output "certificate_arn" {
  description = "ARN of the ACM certificate. Taken from the validation resource so that it is issued before the listeners use it."
  value       = aws_acm_certificate_validation.this.certificate_arn
}

output "fqdn" {
  description = "Fully qualified domain name that points to the ALB."
  value       = aws_route53_record.alb.fqdn
}
