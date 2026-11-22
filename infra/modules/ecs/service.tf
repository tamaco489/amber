# ================================================================================
# Task Definition
# ================================================================================

# 最初のタスク定義
#   - イメージはタグではなくダイジェストで指定する (同じタグで中身が変わっても、動くイメージが変わらないようにする)
#   - 以降のリビジョンは CD が登録するため、サービスは task_definition の変更を Terraform で戻さない
#   - ARM64 (Graviton) で動かす
resource "aws_ecs_task_definition" "api" {
  family                   = "${local.name_prefix}-api"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = aws_iam_role.task_execution.arn
  task_role_arn            = aws_iam_role.task.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }

  # コンテナの設定
  #   - 非 root (distroless の nonroot と同じ UID) で動かし、ルートファイルシステムは読み取り専用にする
  #   - stopTimeout: SIGTERM から SIGKILL までの猶予。アプリの終了待ち (20 秒) より長くする
  #   - コンテナのヘルスチェックは設定しない (distroless に curl がなく、ALB のヘルスチェックで見る)
  container_definitions = jsonencode([
    {
      name                   = "api"
      image                  = var.image
      essential              = true
      user                   = "65532:65532"
      readonlyRootFilesystem = true
      stopTimeout            = var.stop_timeout

      portMappings = [
        {
          containerPort = var.container_port
          protocol      = "tcp"
        },
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.api.name
          awslogs-region        = data.aws_region.current.region
          awslogs-stream-prefix = "api"
        }
      }
    },
  ])
}

# ================================================================================
# Service
# ================================================================================

# api のサービス
#   - プライベートサブネットに置き、パブリック IP は付けない (イメージの pull とログは VPC エンドポイント経由)
#   - タスクは AZ に分散させ、偏ったときは ECS が配置し直す (AZ のリバランス)
#   - ローリング: 最小ヘルシー率 100% / 最大 200% (古いタスクを止める前に、同じ数の新しいタスクを起動する)
#   - ロールバック: タスクが起動できないとき (サーキットブレーカー) と、5xx が増えたとき (アラーム)
#   - タスクは blue のターゲットグループに登録する (リスナーのルールは blue / green を 1 : 0 で転送している)
resource "aws_ecs_service" "api" {
  name                              = "${local.name_prefix}-api"
  cluster                           = aws_ecs_cluster.this.id
  task_definition                   = aws_ecs_task_definition.api.arn
  desired_count                     = var.desired_count
  health_check_grace_period_seconds = var.health_check_grace_period
  availability_zone_rebalancing     = "ENABLED"
  enable_ecs_managed_tags           = true
  propagate_tags                    = "SERVICE"

  capacity_provider_strategy {
    capacity_provider = "FARGATE"
    weight            = 1
  }

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.task.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = var.target_group_arn
    container_name   = "api"
    container_port   = var.container_port
  }

  deployment_minimum_healthy_percent = 100
  deployment_maximum_percent         = 200

  # ベイクタイム: ロールバック付きのアラーム (期間 60 秒) を 1〜2 回判定できる長さにする (provider の型は文字列)
  deployment_configuration {
    strategy             = "ROLLING"
    bake_time_in_minutes = "2"
  }

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  alarms {
    alarm_names = [aws_cloudwatch_metric_alarm.target_5xx.alarm_name]
    enable      = true
    rollback    = true
  }

  # task_definition: CD が新しいリビジョンを登録してデプロイするため
  # desired_count: オートスケーリングが変えるため
  lifecycle {
    ignore_changes = [task_definition, desired_count]
  }

  depends_on = [aws_ecs_cluster_capacity_providers.this]
}

# ================================================================================
# Auto Scaling
# ================================================================================

# タスク数を CPU 使用率に合わせて増減させる (desired_count から max_count まで)
#   - 目標追跡は CloudWatch アラームを 2 つ自動で作る (サービスを消すと一緒に消える)
resource "aws_appautoscaling_target" "api" {
  service_namespace  = "ecs"
  resource_id        = "service/${aws_ecs_cluster.this.name}/${aws_ecs_service.api.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  min_capacity       = var.desired_count
  max_capacity       = var.max_count
}

resource "aws_appautoscaling_policy" "api_cpu" {
  name               = "${local.name_prefix}-api-cpu"
  policy_type        = "TargetTrackingScaling"
  service_namespace  = aws_appautoscaling_target.api.service_namespace
  resource_id        = aws_appautoscaling_target.api.resource_id
  scalable_dimension = aws_appautoscaling_target.api.scalable_dimension

  target_tracking_scaling_policy_configuration {
    target_value = 70

    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
  }
}

# ================================================================================
# Alarm
# ================================================================================

# デプロイ中のロールバックに使うアラーム (ターゲットが返した 5xx)
#   - ALB 全体のメトリクスのため、blue / green のどちらのターゲットグループの 5xx も数える
#   - 1 分間に target_5xx_threshold 件以上で発火する。リクエストがない間はデータがないため、正常として扱う
resource "aws_cloudwatch_metric_alarm" "target_5xx" {
  alarm_name          = "${local.name_prefix}-api-target-5xx"
  alarm_description   = "5xx responses from the api targets. Used to roll back ECS deployments."
  namespace           = "AWS/ApplicationELB"
  metric_name         = "HTTPCode_Target_5XX_Count"
  dimensions          = { LoadBalancer = var.alb_arn_suffix }
  statistic           = "Sum"
  period              = 60
  evaluation_periods  = 1
  threshold           = var.target_5xx_threshold
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
}
