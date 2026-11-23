output "deploy_event_source" {
  description = "Source of the events that the cd-backend workflow sends when it starts a deployment."
  value       = local.deploy_event_source
}

output "deploy_event_detail_type" {
  description = "Detail type of the events that the cd-backend workflow sends when it starts a deployment."
  value       = local.deploy_event_detail_type
}

output "connection_name" {
  description = "Name of the EventBridge connection. Set the Slack bot token to it with aws events update-connection."
  value       = aws_cloudwatch_event_connection.slack.name
}

output "dead_letter_queue_url" {
  description = "URL of the SQS queue that keeps the notifications EventBridge failed to deliver."
  value       = aws_sqs_queue.dead_letter.url
}

output "dead_letter_queue_arn" {
  description = "ARN of the SQS queue for failed notifications. The notification Lambda sends its failed invocations to it."
  value       = aws_sqs_queue.dead_letter.arn
}

output "notifier_function_name" {
  description = "Name of the notification Lambda function. The function itself is not created yet."
  value       = local.notifier_function_name
}

output "notifier_role_arn" {
  description = "ARN of the execution role for the notification Lambda."
  value       = aws_iam_role.notifier.arn
}

output "notifier_log_group_name" {
  description = "Name of the CloudWatch log group for the notification Lambda."
  value       = aws_cloudwatch_log_group.notifier.name
}

output "notifier_state_table_name" {
  description = "Name of the DynamoDB table that keeps the state of the notifications of each deployment."
  value       = aws_dynamodb_table.notifier_state.name
}

output "notifier_state_table_arn" {
  description = "ARN of the DynamoDB table that keeps the state of the notifications of each deployment."
  value       = aws_dynamodb_table.notifier_state.arn
}

output "slack_bot_token_parameter_name" {
  description = "Name of the SSM parameter for the Slack bot token. Set the real token to it with aws ssm put-parameter --overwrite."
  value       = aws_ssm_parameter.slack_bot_token.name
}

output "slack_bot_token_parameter_arn" {
  description = "ARN of the SSM parameter for the Slack bot token."
  value       = aws_ssm_parameter.slack_bot_token.arn
}
