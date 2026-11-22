output "plan_role_arn" {
  description = "ARN of the IAM role that the ci-infra workflow assumes to run terraform plan."
  value       = aws_iam_role.plan.arn
}

output "deploy_role_arn" {
  description = "ARN of the IAM role that the cd-backend workflow assumes to push the image and deploy it to ECS."
  value       = aws_iam_role.deploy.arn
}
