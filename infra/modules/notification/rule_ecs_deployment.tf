# ================================================================================
# ECS Deployment State Change
# ================================================================================

# api のサービスのデプロイの状態の変化 (default のイベントバスに届く)
#   - 届くイベント (detail.eventName) ごとにルールを分け、色と見出しを変える
#     - SERVICE_DEPLOYMENT_IN_PROGRESS
#     - SERVICE_DEPLOYMENT_COMPLETED
#     - SERVICE_DEPLOYMENT_FAILED
#   - eventName は決まった値のため、絞り込みに使う
#   - ロールバックも IN_PROGRESS で届き、違いは reason だけのため、reason を本文に載せて見分ける
#     (reason は説明文で、文面は変わりうるため、ルールの絞り込みには使わない)
# DOC: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_deployment_events.html
locals {
  ecs_deployment_events = {
    in-progress = {
      event_name = "SERVICE_DEPLOYMENT_IN_PROGRESS"
      color      = local.slack_colors.warning
      title      = ":hourglass_flowing_sand: Deployment in progress"
    }
    completed = {
      event_name = "SERVICE_DEPLOYMENT_COMPLETED"
      color      = local.slack_colors.success
      title      = ":white_check_mark: Deployment completed"
    }
    failed = {
      event_name = "SERVICE_DEPLOYMENT_FAILED"
      color      = local.slack_colors.danger
      title      = ":x: Deployment failed"
    }
  }
}

resource "aws_cloudwatch_event_rule" "ecs_deployment" {
  for_each = local.ecs_deployment_events

  name        = "${local.name_prefix}-ecs-deployment-${each.key}"
  description = "${each.value.event_name} of the api service."

  event_pattern = jsonencode({
    source      = ["aws.ecs"]
    detail-type = ["ECS Deployment State Change"]
    resources   = [local.ecs_service_arn]
    detail = {
      eventName = [each.value.event_name]
    }
  })
}

# 本文の組み立て (templates/ の Slack の Block Kit。色の帯を付けるため attachments に入れる)
#   - ${...} は Terraform が plan のときに、<...> は EventBridge がイベントが届いたときに埋める
#   - EventBridge は、取り出した値をエスケープせずに埋め込む。reason に " が入ると本文の JSON が壊れ、その通知は届かない
#   - text は、通知のプレビューに出る文
# DOC: https://docs.slack.dev/reference/block-kit/blocks
resource "aws_cloudwatch_event_target" "ecs_deployment" {
  for_each = local.ecs_deployment_events

  rule      = aws_cloudwatch_event_rule.ecs_deployment[each.key].name
  target_id = "slack"
  arn       = aws_cloudwatch_event_api_destination.slack.arn
  role_arn  = aws_iam_role.invoke_api_destination.arn

  input_transformer {
    input_paths = {
      deploymentId = "$.detail.deploymentId"
      reason       = "$.detail.reason"
      time         = "$.time"
    }

    input_template = templatefile("${path.module}/templates/ecs_deployment.json.tftpl", {
      channel = var.slack_channel_id
      color   = each.value.color
      title   = each.value.title
      service = local.ecs_service_name
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
