locals {
  name_prefix = "${var.env}-amber"

  account_id = data.aws_caller_identity.current.account_id
  region     = data.aws_region.current.region

  # 通知する ECS のサービス
  #   - サービスは enable_runtime = false の間は存在しないため、modules/ecs の出力ではなく、同じ命名規則で組み立てた ARN を使う
  #     (modules/ecs の名前を変えたら合わせる)
  ecs_service_arn = "arn:aws:ecs:${local.region}:${local.account_id}:service/${local.name_prefix}/${local.name_prefix}-api"

  # cd-backend が送る、デプロイを始めたことのイベント
  #   - source は aws. で始められない (AWS のサービスのイベントと区別するため)
  #   - cd-backend の notify-deploy-started.sh と、deploy 用のロールの PutEvents の条件で、同じ値を使う
  deploy_event_source      = "amber.cd-backend"
  deploy_event_detail_type = "Deployment Started"

  # 通知のルール (ロールの信頼条件と、デッドレターキューへの送信の許可に使う)
  rule_arns = concat(
    [for rule in aws_cloudwatch_event_rule.ecs_deployment : rule.arn],
    [aws_cloudwatch_event_rule.deploy_started.arn],
  )

  # Slack の通知の色の帯 (Slack のブランドカラー)
  slack_colors = {
    info    = "#36C5F0"
    success = "#2EB67D"
    warning = "#ECB22E"
    danger  = "#E01E5A"
  }

  # 届けられなかったときの再試行
  #   - 1 時間より古い通知は、届いても役に立たないため捨てる (デッドレターキューに入る)
  max_event_age_seconds = 60 * 60 # 1 時間
  max_retry_attempts    = 10
}
