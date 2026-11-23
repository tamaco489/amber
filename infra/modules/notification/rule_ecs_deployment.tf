# ================================================================================
# ECS Deployment State Change
# ================================================================================

# api のサービスのデプロイの状態の変化 (default のイベントバスに届く)
#   - 届くイベント (detail.eventName) ごとにルールを分ける。eventName は決まった値のため、絞り込みに使う
#     - SERVICE_DEPLOYMENT_IN_PROGRESS
#     - SERVICE_DEPLOYMENT_COMPLETED
#     - SERVICE_DEPLOYMENT_FAILED
#   - ロールバックも IN_PROGRESS で届き、違いは reason だけ。見分けるのは Lambda の側 (backend/internal/deploy)
#     (reason は説明文で、文面は変わりうるため、ルールの絞り込みには使わない)
# DOC: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_deployment_events.html
locals {
  ecs_deployment_events = {
    in-progress = "SERVICE_DEPLOYMENT_IN_PROGRESS"
    completed   = "SERVICE_DEPLOYMENT_COMPLETED"
    failed      = "SERVICE_DEPLOYMENT_FAILED"
  }
}

resource "aws_cloudwatch_event_rule" "ecs_deployment" {
  for_each = local.ecs_deployment_events

  name        = "${local.name_prefix}-ecs-deployment-${each.key}"
  description = "${each.value} of the api service."

  event_pattern = jsonencode({
    source      = ["aws.ecs"]
    detail-type = ["ECS Deployment State Change"]
    resources   = [local.ecs_service_arn]
    detail = {
      eventName = [each.value]
    }
  })
}

# ターゲットは通知の Lambda (lambda.tf)。入力変換はせず、イベントをそのまま渡す (Lambda が EventBridge のイベントの形で読む)
#   - EventBridge は Lambda を非同期で呼び、Lambda が呼び出しを受け付けた時点で成功とみなす
#   - そのため、下の再試行と DLQ が受け持つのは、Lambda に届けられなかったとき (スロットリング、権限の不足など) だけ
#   - 関数がエラーを返したとき (Slack が ok: false を返したときを含む) の再試行と送り先は、Lambda の側 (lambda.tf の非同期の呼び出しの設定) で決める
#   - Lambda のターゲットはロールを使わない。呼ぶ許可は、関数のリソースベースのポリシー (aws_lambda_permission) で与える
# DOC: https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-rule-retry-policy.html
# DOC: https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-rule-dlq.html
resource "aws_cloudwatch_event_target" "ecs_deployment" {
  for_each = local.ecs_deployment_events

  rule      = aws_cloudwatch_event_rule.ecs_deployment[each.key].name
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

# EventBridge に、このルールからだけ関数を呼ぶことを許す (source_arn でルールを絞る)
# DOC: https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-use-resource-based.html#eb-lambda-permissions
resource "aws_lambda_permission" "ecs_deployment" {
  for_each = local.ecs_deployment_events

  statement_id  = "AllowEventBridge-${aws_cloudwatch_event_rule.ecs_deployment[each.key].name}"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.notifier.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.ecs_deployment[each.key].arn
}
