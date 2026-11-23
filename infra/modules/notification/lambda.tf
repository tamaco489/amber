# ================================================================================
# Lambda Logs
# ================================================================================

# 通知の Lambda のログ
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

# ================================================================================
# Lambda Function
# ================================================================================

# 関数を作るときだけに使う、仮のコード (bootstrap だけを入れた zip)
#   - 本物のコードは、cd-notifier のワークフローが main への push のたびにビルドして置く (aws lambda update-function-code)
#   - 仮のコードは動かない (呼ぶと失敗する)。apply のあと、cd-notifier を流すまで、関数は使えない
#   - zip は .terraform/ (Git の対象外) に作る
data "archive_file" "notifier_placeholder" {
  type        = "zip"
  output_path = "${path.root}/.terraform/tmp/notifier-placeholder.zip"

  source {
    filename = "bootstrap"
    content  = "#!/bin/sh\necho 'placeholder: deploy the code with the cd-notifier workflow' >&2\nexit 1\n"
  }
}

# デプロイの EventBridge のイベントを受け、デプロイごとに 1 つの Slack のスレッドにまとめる関数 (backend/cmd/notifier)
#   - provided.al2023 の arm64。実行ファイルの名前は bootstrap (just backend::build-notifier と cd-notifier が同じ条件でビルドする)
#   - タイムアウトの 30 秒は、手順の印の期限 (backend/internal/dynamodb の DefaultLeaseTime、60 秒) より短くする
#     (期限より長く動くと、再試行の呼び出しが同じ手順を並べて進めるため)
#   - 置いておくだけでは料金はかからない (リクエストと実行時間の課金)。そのため enable_runtime では消さない
#   - コードは cd-notifier が置くため、Terraform は filename と source_code_hash の変更を無視する
#     (無視しないと、apply のたびに仮のコードへ戻る)
# DOC: https://docs.aws.amazon.com/lambda/latest/dg/golang-package.html
# DOC: https://aws.amazon.com/lambda/pricing/
resource "aws_lambda_function" "notifier" {
  function_name    = local.notifier_function_name
  description      = "Groups the deployment notifications into one Slack thread per deployment. The code is deployed by the cd-notifier workflow, not Terraform."
  role             = aws_iam_role.notifier.arn
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  handler          = "bootstrap"
  timeout          = 30
  memory_size      = 128
  filename         = data.archive_file.notifier_placeholder.output_path
  source_code_hash = data.archive_file.notifier_placeholder.output_base64sha256

  environment {
    variables = {
      TABLE_NAME            = aws_dynamodb_table.notifier_state.name
      SLACK_TOKEN_PARAMETER = aws_ssm_parameter.slack_bot_token.name
      SLACK_CHANNEL_ID      = var.slack_channel_id
      SERVICE_NAME          = local.ecs_service_name
    }
  }

  logging_config {
    log_format = "Text"
    log_group  = aws_cloudwatch_log_group.notifier.name
  }

  lifecycle {
    ignore_changes = [filename, source_code_hash]
  }

  # 関数より先にロググループを作る (Lambda に、保持期間が無期限のロググループを作らせないため)
  depends_on = [aws_cloudwatch_log_group.notifier]
}

# 非同期の呼び出し (EventBridge から) の再試行と、失敗したときの送り先
#   - 関数がエラーを返したとき (Slack が ok: false を返したときを含む) は、Lambda が最大 2 回まで再試行する
#     EventBridge は、Lambda が非同期の呼び出しを受け付けた時点で成功とみなすため、EventBridge 側の再試行とデッドレターキューには乗らない
#   - 1 時間より古いイベントは捨てる (ルールの再試行と同じ)
#   - 再試行しても失敗したものは、ルールと同じデッドレターキューに送る (送信の許可は実行ロールにある)
# DOC: https://docs.aws.amazon.com/lambda/latest/dg/invocation-async-error-handling.html
# DOC: https://docs.aws.amazon.com/lambda/latest/dg/invocation-async-retain-records.html
resource "aws_lambda_function_event_invoke_config" "notifier" {
  function_name                = aws_lambda_function.notifier.function_name
  maximum_retry_attempts       = 2
  maximum_event_age_in_seconds = local.max_event_age_seconds

  destination_config {
    on_failure {
      destination = aws_sqs_queue.dead_letter.arn
    }
  }
}
