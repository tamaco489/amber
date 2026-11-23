# ================================================================================
# Dead Letter Queue
# ================================================================================

# EventBridge が Slack に届けられなかった通知を残すキュー
#   - 再試行しても届かなかったもの (Slack が 5xx や 429 を返し続けた、本文の JSON が壊れていた、など) が入る
#   - Slack は、トークンやチャンネルの誤りでも HTTP 200 (本文の ok が false) を返すため、それはここに入らない
#   - 置いておくだけでは料金はかからない (リクエストごとの課金)
resource "aws_sqs_queue" "dead_letter" {
  name                      = "${local.name_prefix}-notification-dlq"
  message_retention_seconds = 3 * 24 * 60 * 60 # 3 日 (検証の合間に確かめられれば足りる。コンテナのログと同じ)
  sqs_managed_sse_enabled   = true
}

resource "aws_sqs_queue_policy" "dead_letter" {
  queue_url = aws_sqs_queue.dead_letter.id
  policy    = data.aws_iam_policy_document.dead_letter.json
}

data "aws_iam_policy_document" "dead_letter" {
  # 通知のルールからの送信だけを許す
  statement {
    sid       = "AllowNotificationRules"
    actions   = ["sqs:SendMessage"]
    resources = [aws_sqs_queue.dead_letter.arn]

    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }

    condition {
      test     = "ArnEquals"
      variable = "aws:SourceArn"
      values   = local.rule_arns
    }
  }

  # HTTPS 以外 (aws:SecureTransport = false) のリクエストは拒否する
  statement {
    sid       = "DenyInsecureTransport"
    effect    = "Deny"
    actions   = ["sqs:*"]
    resources = [aws_sqs_queue.dead_letter.arn]

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}
