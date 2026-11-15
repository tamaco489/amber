output "dns_name" {
  description = "DNS name of the ALB."
  value       = aws_lb.this.dns_name
}

output "zone_id" {
  description = "Hosted zone ID of the ALB, used for alias records."
  value       = aws_lb.this.zone_id
}

output "security_group_id" {
  description = "ID of the security group of the ALB. Allow it as the source on the task security group."
  value       = aws_security_group.alb.id
}

output "target_group_arns" {
  description = "ARNs of the target groups, keyed by blue and green."
  value       = { for k, tg in aws_lb_target_group.this : k => tg.arn }
}

output "listener_rule_arns" {
  description = "ARNs of the forwarding rules, keyed by prod (443) and test (8443). ECS blue/green deployments rewrite their weights."
  value       = { for k, r in aws_lb_listener_rule.forward : k => r.arn }
}
