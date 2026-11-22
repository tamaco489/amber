locals {
  name_prefix = "${var.env}-amber"

  account_id = data.aws_caller_identity.current.account_id
  region     = data.aws_region.current.region

  # deploy 用のロールが操作する ECS のリソース
  #   - ECS のクラスター、サービス、タスクのロールは enable_runtime = false の間は存在しないため、
  #     modules/ecs の出力ではなく、同じ命名規則で組み立てた ARN を使う (modules/ecs の名前を変えたら合わせる)
  ecs_service_arn         = "arn:aws:ecs:${local.region}:${local.account_id}:service/${local.name_prefix}/${local.name_prefix}-api"
  ecs_task_definition_arn = "arn:aws:ecs:${local.region}:${local.account_id}:task-definition/${local.name_prefix}-api:*"
  ecs_task_role_arns = [
    "arn:aws:iam::${local.account_id}:role/${local.name_prefix}-ecs-task-execution",
    "arn:aws:iam::${local.account_id}:role/${local.name_prefix}-ecs-task",
  ]
}
