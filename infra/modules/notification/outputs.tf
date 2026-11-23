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
