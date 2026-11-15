# ================================================================================
# IAM Roles
# ================================================================================

# ECS のタスクとサービスが引き受けるロールの信頼ポリシー
data "aws_iam_policy_document" "ecs_tasks_trust" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "ecs_trust" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs.amazonaws.com"]
    }
  }
}

# タスク実行ロール
#   - ECS のエージェントが、タスクを起動するときに使う (アプリは使わない)
#   - 権限: ECR からのイメージの pull、CloudWatch Logs へのログの出力
resource "aws_iam_role" "task_execution" {
  name               = "${local.name_prefix}-ecs-task-execution"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_trust.json
}

resource "aws_iam_role_policy_attachment" "task_execution" {
  role       = aws_iam_role.task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# タスクロール
#   - アプリが AWS の API を呼ぶときに使う
#   - 検証用のアプリは AWS の API を呼ばないため、権限は付けない
resource "aws_iam_role" "task" {
  name               = "${local.name_prefix}-ecs-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_trust.json
}

# インフラストラクチャロール
#   - ECS のブルー/グリーン、カナリア、リニアで、ECS がリスナーのルールの重みとターゲットグループを書き換えるときに使う
#   - ローリングでは使わない。方式を切り替えるときにサービスの設定だけで済むよう、先に作っておく
resource "aws_iam_role" "infrastructure" {
  name               = "${local.name_prefix}-ecs-infrastructure"
  assume_role_policy = data.aws_iam_policy_document.ecs_trust.json
}

resource "aws_iam_role_policy_attachment" "infrastructure" {
  role       = aws_iam_role.infrastructure.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonECSInfrastructureRolePolicyForLoadBalancers"
}
