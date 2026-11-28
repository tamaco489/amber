# ================================================================================
# Dead Letter Queue
# ================================================================================

# 通知に失敗したイベントを残すキュー。2 か所から入る
#   - ルール: 再試行しても、EventBridge が Lambda に届けられなかったイベント (スロットリング、呼ぶ許可の不足など)
#     メッセージの属性 (ERROR_CODE、ERROR_MESSAGE) に理由が付く
#   - Lambda: 再試行しても、関数がエラーを返した呼び出し (Slack が ok: false や 5xx を返した、など)
#     本文は呼び出しの記録 (requestContext、元のイベント、関数の応答)
#   - 置いておくだけでは料金はかからない (リクエストごとの課金)
# DOC: https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-rule-dlq.html
# DOC: https://docs.aws.amazon.com/lambda/latest/dg/invocation-async-retain-records.html
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
  #   - Lambda からの送信は、実行ロールの許可で届く (同じアカウントのため。lambda.tf)
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
