# ================================================================================
# Lambda Logs
# ================================================================================

# 通知の Lambda のログ (関数は、まだ置かない)
#   - Lambda が自動で作ると保持期間が無期限になるため、関数より先に作る
#   - 検証の間だけ見られればよいため、コンテナのログと同じ 3 日にする
resource "aws_cloudwatch_log_group" "notifier" {
  name              = "/aws/lambda/${local.notifier_function_name}"
  retention_in_days = 3
}

# ================================================================================
# Lambda Execution Role
# ================================================================================

# 通知の Lambda の実行ロール
#   - 信頼条件: Lambda から。ロールを関数に付けられるのは、このアカウントで iam:PassRole を持つものだけ
#   - 権限: 状態のテーブルの読み書き、ボットトークンの読み取り、ログの書き込み、失敗したときのデッドレターキューへの送信
resource "aws_iam_role" "notifier" {
  name               = "${local.name_prefix}-notification-lambda"
  assume_role_policy = data.aws_iam_policy_document.notifier_trust.json
}

data "aws_iam_policy_document" "notifier_trust" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy" "notifier" {
  name   = "notifier"
  role   = aws_iam_role.notifier.id
  policy = data.aws_iam_policy_document.notifier.json
}

# ボットトークンは AWS マネージドキー (aws/ssm) で暗号化している。このキーのキーポリシーが、
# アカウント内のプリンシパルに Parameter Store 経由の復号を許しているため、kms:Decrypt は付けない
# DOC: https://docs.aws.amazon.com/systems-manager/latest/userguide/ps-restrict-decryption.html
data "aws_iam_policy_document" "notifier" {
  statement {
    sid = "ReadWriteState"
    actions = [
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:UpdateItem",
      "dynamodb:DeleteItem",
    ]
    resources = [aws_dynamodb_table.notifier_state.arn]
  }

  statement {
    sid       = "ReadSlackBotToken"
    actions   = ["ssm:GetParameter"]
    resources = [aws_ssm_parameter.slack_bot_token.arn]
  }

  # このロググループの中のログストリームだけ (Lambda のコンソールが作るロールと同じ形)
  statement {
    sid = "WriteLogs"
    actions = [
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    resources = ["${aws_cloudwatch_log_group.notifier.arn}:*"]
  }

  # 非同期の呼び出しが失敗したときの送り先 (デッドレターキュー) への送信
  #   - 同じアカウントのため、キューのポリシーに許可がなくても、このロールの許可だけで送れる
  statement {
    sid       = "SendToDeadLetterQueue"
    actions   = ["sqs:SendMessage"]
    resources = [aws_sqs_queue.dead_letter.arn]
  }
}
