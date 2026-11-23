# ================================================================================
# Deploy Role
# ================================================================================

# cd-backend でイメージを push し、ECS にデプロイするロール
#   - 信頼条件: このリポジトリの v* のタグから発行されたトークンだけ (タグの push と、タグを選んだ workflow_dispatch)
#   - ブランチから実行すると sub が refs/heads/... になるため、引き受けられない
#   - 権限: ECR への push、タスク定義の登録、サービスの更新、タスクのロールを ECS に渡すこと、デプロイを始めたことの通知
resource "aws_iam_role" "deploy" {
  name               = "${local.name_prefix}-github-actions-deploy"
  assume_role_policy = data.aws_iam_policy_document.deploy_trust.json
}

data "aws_iam_policy_document" "deploy_trust" {
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
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      values   = ["${var.github_oidc_sub_prefix}:ref:refs/tags/v*"]
    }
  }
}

resource "aws_iam_role_policy" "deploy" {
  name   = "deploy"
  role   = aws_iam_role.deploy.id
  policy = data.aws_iam_policy_document.deploy.json
}

# リソースに * を使うのは、リソースを絞れない API (GetAuthorizationToken、DescribeTaskDefinition) だけにする
# ほかはリポジトリ、タスク定義のファミリー、サービス、ロールの ARN に絞る
# DOC: https://docs.aws.amazon.com/service-authorization/latest/reference/list_ecs.html
data "aws_iam_policy_document" "deploy" {
  # ECR へのログイン (リソースを指定できない API)
  statement {
    sid       = "EcrLogin"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }

  # api のリポジトリへの push と、push 済みのイメージの確認 (workflow_dispatch で同じタグを流し直すとき)
  statement {
    sid = "EcrPush"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:BatchGetImage",
      "ecr:CompleteLayerUpload",
      "ecr:DescribeImages",
      "ecr:InitiateLayerUpload",
      "ecr:PutImage",
      "ecr:UploadLayerPart",
    ]
    resources = [var.ecr_repository_arn]
  }

  # 今のタスク定義の読み取り (リソースを指定できない API)
  statement {
    sid       = "DescribeTaskDefinition"
    actions   = ["ecs:DescribeTaskDefinition"]
    resources = ["*"]
  }

  # api のファミリーへの、新しいリビジョンの登録
  statement {
    sid       = "RegisterTaskDefinition"
    actions   = ["ecs:RegisterTaskDefinition"]
    resources = [local.ecs_task_definition_arn]
  }

  # api のサービスの読み取りと更新
  statement {
    sid = "Service"
    actions = [
      "ecs:DescribeServices",
      "ecs:UpdateService",
    ]
    resources = [local.ecs_service_arn]
  }

  # タスク定義に書いたタスク実行ロールとタスクロールを、ECS に渡す (登録に必要)
  statement {
    sid       = "PassTaskRoles"
    actions   = ["iam:PassRole"]
    resources = local.ecs_task_role_arns

    condition {
      test     = "StringEquals"
      variable = "iam:PassedToService"
      values   = ["ecs-tasks.amazonaws.com"]
    }
  }

  # デプロイを始めたことのイベントを、default のイベントバスに送る (Slack への通知)
  #   - source と detail-type を、通知のルールが拾う値に限る (ほかのイベントを装って送れないようにする)
  statement {
    sid       = "PutDeployStartedEvent"
    actions   = ["events:PutEvents"]
    resources = [local.default_event_bus_arn]

    condition {
      test     = "StringEquals"
      variable = "events:source"
      values   = [var.deploy_event_source]
    }

    condition {
      test     = "StringEquals"
      variable = "events:detail-type"
      values   = [var.deploy_event_detail_type]
    }
  }
}
