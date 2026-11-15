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

output "api_repository_url" {
  description = "URL of the ECR repository for the api image."
  value       = module.ecr_api.repository_url
}
