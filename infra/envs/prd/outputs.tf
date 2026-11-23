output "vpc_id" {
  description = "ID of the VPC."
  value       = module.network.vpc_id
}

output "public_subnet_ids" {
  description = "IDs of the public subnets."
  value       = module.network.public_subnet_ids
}

output "private_subnet_ids" {
  description = "IDs of the private subnets."
  value       = module.network.private_subnet_ids
}

output "plan_role_arn" {
  description = "ARN of the IAM role for the ci-infra plan. Register it as the AWS_PLAN_ROLE_ARN repository variable."
  value       = module.github_oidc.plan_role_arn
}

output "deploy_role_arn" {
  description = "ARN of the IAM role for the cd-backend deploy. Register it as the AWS_DEPLOY_ROLE_ARN repository variable."
  value       = module.github_oidc.deploy_role_arn
}

output "notifier_deploy_role_arn" {
  description = "ARN of the IAM role for the cd-notifier deploy. Register it as the AWS_NOTIFIER_DEPLOY_ROLE_ARN repository variable."
  value       = module.github_oidc.notifier_deploy_role_arn
}

output "api_repository_url" {
  description = "URL of the ECR repository for the api image."
  value       = module.ecr_api.repository_url
}

output "app_url" {
  description = "URL of the application. Null while enable_runtime is false."
  value       = var.enable_runtime ? "https://${module.dns[0].fqdn}" : null
}

output "ecs_cluster_name" {
  description = "Name of the ECS cluster. Null while enable_runtime is false."
  value       = var.enable_runtime ? module.ecs[0].cluster_name : null
}

output "ecs_service_name" {
  description = "Name of the ECS service. Null while enable_runtime is false."
  value       = var.enable_runtime ? module.ecs[0].service_name : null
}

output "notification_dead_letter_queue_url" {
  description = "URL of the SQS queue that keeps the failed notifications (events not delivered to the notification Lambda, and invocations that failed after its retries)."
  value       = module.notification.dead_letter_queue_url
}

output "notification_slack_bot_token_parameter_name" {
  description = "Name of the SSM parameter for the Slack bot token of the notification Lambda. Set the real token to it with aws ssm put-parameter --overwrite."
  value       = module.notification.slack_bot_token_parameter_name
}

output "notification_log_group_name" {
  description = "Name of the CloudWatch log group for the notification Lambda."
  value       = module.notification.notifier_log_group_name
}

output "notification_function_name" {
  description = "Name of the notification Lambda function. The cd-notifier workflow deploys its code."
  value       = module.notification.notifier_function_name
}
