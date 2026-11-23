# ================================================================================
# Deployment Started (cd-backend)
# ================================================================================

# cd-backend がデプロイを始めたこと (update-service のあとに PutEvents で送る)
resource "aws_cloudwatch_event_rule" "deploy_started" {
  name        = "${local.name_prefix}-deploy-started"
  description = "Deployments started by the cd-backend workflow."

  event_pattern = jsonencode({
    source      = [local.deploy_event_source]
    detail-type = [local.deploy_event_detail_type]
  })
}

# 本文の組み立ては rule_ecs_deployment.tf と同じ (templates/deploy_started.json.tftpl)
resource "aws_cloudwatch_event_target" "deploy_started" {
  rule      = aws_cloudwatch_event_rule.deploy_started.name
  target_id = "slack"
  arn       = aws_cloudwatch_event_api_destination.slack.arn
  role_arn  = aws_iam_role.invoke_api_destination.arn

  input_transformer {
    input_paths = {
      version        = "$.detail.version"
      deploymentId   = "$.detail.deploymentId"
      taskDefinition = "$.detail.taskDefinition"
      pushed         = "$.detail.pushed"
      actor          = "$.detail.actor"
      runUrl         = "$.detail.runUrl"
    }

    input_template = templatefile("${path.module}/templates/deploy_started.json.tftpl", {
      channel = var.slack_channel_id
      color   = local.slack_colors.info
    })
  }

  retry_policy {
    maximum_event_age_in_seconds = local.max_event_age_seconds
    maximum_retry_attempts       = local.max_retry_attempts
  }

  dead_letter_config {
    arn = aws_sqs_queue.dead_letter.arn
  }
}
