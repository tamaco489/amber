# ================================================================================
# Plan Role
# ================================================================================

# PR の ci-infra で terraform plan を実行するロール
#   - 信頼条件: このリポジトリの pull_request イベントから発行されたトークンだけ
#   - フォークからの PR には OIDC のトークンが発行されないため、外部の PR からは引き受けられない
#   - 権限: 読み取り専用 (plan は AWS の現状を読むだけで、変更しない)
resource "aws_iam_role" "plan" {
  name               = "${local.name_prefix}-github-actions-plan"
  assume_role_policy = data.aws_iam_policy_document.plan_trust.json
}

data "aws_iam_policy_document" "plan_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [data.aws_iam_openid_connect_provider.github.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:sub"
      values   = ["repo:${var.github_repository}:pull_request"]
    }
  }
}

# AWS の現状を読むための権限
#   - plan は、管理しているすべてのリソースの設定を読むため、サービスを限定しない AWS 管理ポリシーを使う
resource "aws_iam_role_policy_attachment" "plan_read_only" {
  role       = aws_iam_role.plan.name
  policy_arn = "arn:aws:iam::aws:policy/ReadOnlyAccess"
}

# state を読み、ロックを取るための権限
#   - ロックは S3 のネイティブロック (use_lockfile) で、state と同じ場所に .tflock を作って消す
#   - 書き込みを許すのは .tflock だけで、state 自体は書き換えられない
resource "aws_iam_role_policy" "plan_tfstate" {
  name   = "tfstate"
  role   = aws_iam_role.plan.id
  policy = data.aws_iam_policy_document.plan_tfstate.json
}

data "aws_iam_policy_document" "plan_tfstate" {
  statement {
    sid       = "ListStateBucket"
    actions   = ["s3:ListBucket"]
    resources = ["arn:aws:s3:::${var.tfstate_bucket}"]
  }

  statement {
    sid       = "ReadState"
    actions   = ["s3:GetObject"]
    resources = ["arn:aws:s3:::${var.tfstate_bucket}/${var.tfstate_key}"]
  }

  statement {
    sid       = "ManageLock"
    actions   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
    resources = ["arn:aws:s3:::${var.tfstate_bucket}/${var.tfstate_key}.tflock"]
  }
}
