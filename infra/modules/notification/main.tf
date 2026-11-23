# ================================================================================
# Connection
# ================================================================================

# Slack の API を呼ぶときの認証 (ボットトークンを Authorization ヘッダーに付ける)
#   - トークンは EventBridge が Secrets Manager に保存する。その料金は API 送信先の料金に含まれる
#   - Terraform ではダミー値で作り、本物のトークンは aws events update-connection で入れる (state に本物の値を残さないため)
#   - 入れたあとに Terraform がダミー値へ戻さないよう、auth_parameters の変更は無視する
resource "aws_cloudwatch_event_connection" "slack" {
  name               = "${local.name_prefix}-slack"
  description        = "Slack bot token for posting deployment notifications. The real token is set with the AWS CLI, not Terraform."
  authorization_type = "API_KEY"

  auth_parameters {
    api_key {
      key   = "Authorization"
      value = "Bearer dummy"
    }
  }

  lifecycle {
    ignore_changes = [auth_parameters]
  }
}

# ================================================================================
# API Destination
# ================================================================================

# Slack の chat.postMessage
#   - 本文は、ルールのターゲットの入力変換で {"channel": ..., "text": ...} の JSON に組む
#   - chat.postMessage は、1 つのチャンネルにおおむね 1 秒に 1 件までのため、呼び出しも 1 秒に 1 回までにする
resource "aws_cloudwatch_event_api_destination" "slack" {
  name                             = "${local.name_prefix}-slack-post-message"
  description                      = "Slack chat.postMessage"
  invocation_endpoint              = "https://slack.com/api/chat.postMessage"
  http_method                      = "POST"
  invocation_rate_limit_per_second = 1
  connection_arn                   = aws_cloudwatch_event_connection.slack.arn
}

# ================================================================================
# Role
# ================================================================================

# EventBridge がルールのターゲット (API 送信先) を呼ぶためのロール
#   - 信頼条件: このアカウントの、通知のルールからだけ
#   - 権限: 通知の API 送信先を呼ぶことだけ
resource "aws_iam_role" "invoke_api_destination" {
  name               = "${local.name_prefix}-notification-invoke"
  assume_role_policy = data.aws_iam_policy_document.invoke_api_destination_trust.json
}

data "aws_iam_policy_document" "invoke_api_destination_trust" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [local.account_id]
    }

    condition {
      test     = "ArnEquals"
      variable = "aws:SourceArn"
      values   = local.rule_arns
    }
  }
}

resource "aws_iam_role_policy" "invoke_api_destination" {
  name   = "invoke-api-destination"
  role   = aws_iam_role.invoke_api_destination.id
  policy = data.aws_iam_policy_document.invoke_api_destination.json
}

data "aws_iam_policy_document" "invoke_api_destination" {
  statement {
    sid       = "InvokeSlack"
    actions   = ["events:InvokeApiDestination"]
    resources = [aws_cloudwatch_event_api_destination.slack.arn]
  }
}

# ================================================================================
# Account
# ================================================================================

data "aws_caller_identity" "current" {}

data "aws_region" "current" {}
