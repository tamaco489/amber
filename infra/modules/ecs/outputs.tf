output "cluster_name" {
  description = "Name of the ECS cluster."
  value       = aws_ecs_cluster.this.name
}

output "service_name" {
  description = "Name of the ECS service."
  value       = aws_ecs_service.api.name
}

output "task_execution_role_arn" {
  description = "ARN of the task execution role. The CD workflow passes it when registering task definitions."
  value       = aws_iam_role.task_execution.arn
}

output "task_role_arn" {
  description = "ARN of the task role."
  value       = aws_iam_role.task.arn
}

output "infrastructure_role_arn" {
  description = "ARN of the infrastructure role that ECS uses to rewrite the listener rules in blue/green deployments."
  value       = aws_iam_role.infrastructure.arn
}
