output "plan_role_arn" {
  description = "ARN of the IAM role that the ci-infra workflow assumes to run terraform plan."
  value       = aws_iam_role.plan.arn
}

output "deploy_role_arn" {
  description = "ARN of the IAM role that the cd-backend workflow assumes to push the image and deploy it to ECS."
  value       = aws_iam_role.deploy.arn
}

output "notifier_deploy_role_arn" {
  description = "ARN of the IAM role that the cd-notifier workflow assumes to update the code of the notification Lambda."
  value       = aws_iam_role.notifier_deploy.arn
}
