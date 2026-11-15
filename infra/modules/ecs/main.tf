# ================================================================================
# Cluster
# ================================================================================

# タスクを動かすクラスター
#   - Container Insights は料金がかかるため使わない (性能のデータを CloudWatch Logs に送り、そこからメトリクスを作るため、ログとメトリクスの両方に課金される)
#   - サービスの CPU とメモリの使用率は、Container Insights がなくても標準のメトリクスで見られ、オートスケーリングにはそれで足りる
# Ref: https://aws.amazon.com/cloudwatch/pricing/
# Ref: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/available-metrics.html
resource "aws_ecs_cluster" "this" {
  name = local.name_prefix

  setting {
    name  = "containerInsights"
    value = "disabled"
  }
}

# サービスが使えるキャパシティプロバイダー
#   - 今は FARGATE だけを使う。FARGATE_SPOT は必要になったら足す
resource "aws_ecs_cluster_capacity_providers" "this" {
  cluster_name       = aws_ecs_cluster.this.name
  capacity_providers = ["FARGATE"]

  default_capacity_provider_strategy {
    capacity_provider = "FARGATE"
    weight            = 1
  }
}

# ================================================================================
# Logs
# ================================================================================

# コンテナのログ (awslogs ドライバーで、インターフェース型エンドポイント経由で送る)
#   - 検証の間だけ見られればよいため、保持期間は短くする
resource "aws_cloudwatch_log_group" "api" {
  name              = "/ecs/${local.name_prefix}-api"
  retention_in_days = var.log_retention_days
}

data "aws_region" "current" {}
