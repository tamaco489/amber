output "plan_role_arn" {
  description = "ARN of the IAM role that the ci-infra workflow assumes to run terraform plan."
  value       = aws_iam_role.plan.arn
}
