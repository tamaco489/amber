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

# ターゲット、再試行、呼ぶ許可の考え方は rule_ecs_deployment.tf と同じ
resource "aws_cloudwatch_event_target" "deploy_started" {
  rule      = aws_cloudwatch_event_rule.deploy_started.name
  target_id = "slack"
  arn       = aws_lambda_function.notifier.arn

  retry_policy {
    maximum_event_age_in_seconds = local.max_event_age_seconds
    maximum_retry_attempts       = local.max_retry_attempts
  }

  dead_letter_config {
    arn = aws_sqs_queue.dead_letter.arn
  }
}

resource "aws_lambda_permission" "deploy_started" {
  statement_id  = "AllowEventBridge-${aws_cloudwatch_event_rule.deploy_started.name}"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.notifier.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.deploy_started.arn
}
