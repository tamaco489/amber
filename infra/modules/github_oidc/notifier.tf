# ================================================================================
# Notifier Deploy Role
# ================================================================================

# cd-notifier で、通知の Lambda のコードを置き換えるロール
#   - 信頼条件: このリポジトリの main のブランチから発行されたトークンだけ (main への push と、main を選んだ workflow_dispatch)
#   - ほかのブランチやタグから実行すると sub が一致しないため、引き受けられない
#   - 権限: 通知の Lambda の 1 つの関数の、コードの置き換えと、置き換えが終わるまでの待ち合わせだけ
#   - cd-backend の deploy 用のロール (v* のタグを信頼する) とは、信頼する ref が違うため分ける
resource "aws_iam_role" "notifier_deploy" {
  name               = "${local.name_prefix}-github-actions-notifier"
  assume_role_policy = data.aws_iam_policy_document.notifier_deploy_trust.json
}

data "aws_iam_policy_document" "notifier_deploy_trust" {
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
      values   = ["${var.github_oidc_sub_prefix}:ref:refs/heads/main"]
    }
  }
}

resource "aws_iam_role_policy" "notifier_deploy" {
  name   = "notifier-deploy"
  role   = aws_iam_role.notifier_deploy.id
  policy = data.aws_iam_policy_document.notifier_deploy.json
}

# 権限の名前は、API の名前に lambda: を付けたもの
#   - UpdateFunctionCode: zip でコードを置き換える
#   - GetFunctionConfiguration: aws lambda wait function-updated が、LastUpdateStatus を読むのに使う
# DOC: https://docs.aws.amazon.com/lambda/latest/dg/lambda-api-permissions-ref.html
# DOC: https://docs.aws.amazon.com/cli/latest/reference/lambda/wait/function-updated.html
data "aws_iam_policy_document" "notifier_deploy" {
  statement {
    sid = "DeployNotifierCode"
    actions = [
      "lambda:UpdateFunctionCode",
      "lambda:GetFunctionConfiguration",
    ]
    resources = [var.notifier_function_arn]
  }
}
