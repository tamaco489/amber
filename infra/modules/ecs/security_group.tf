# ================================================================================
# Security Group
# ================================================================================

# S3 のプレフィックスリスト (AWS が管理する、S3 の IP アドレスの範囲)
#   - イメージのレイヤーの実体は S3 にあり、ゲートウェイ型エンドポイント経由で取得する
data "aws_ec2_managed_prefix_list" "s3" {
  name = "com.amazonaws.${data.aws_region.current.region}.s3"
}

# タスクのセキュリティグループ
#   - 内向き: ALB からのアプリのポートだけ
#   - 外向き: VPC の中のインターフェース型エンドポイント (ECR と CloudWatch Logs) と、S3 への HTTPS だけ
#   - インターネットへの経路はないため、外向きを開けても外には出られないが、意図を明示するため宛先を絞る
resource "aws_security_group" "task" {
  name        = "${local.name_prefix}-ecs-task-sg"
  description = "Allow the application port from the ALB and HTTPS to the VPC endpoints"
  vpc_id      = var.vpc_id

  tags = {
    Name = "${local.name_prefix}-ecs-task-sg"
  }
}

# ALB からアプリのポート (8080) への通信を受け付ける
resource "aws_vpc_security_group_ingress_rule" "task_from_alb" {
  security_group_id            = aws_security_group.task.id
  description                  = "Application port from the ALB"
  ip_protocol                  = "tcp"
  from_port                    = var.container_port
  to_port                      = var.container_port
  referenced_security_group_id = var.alb_security_group_id
}

# インターフェース型エンドポイント (ECR と CloudWatch Logs) へ HTTPS で出る
resource "aws_vpc_security_group_egress_rule" "task_to_endpoints" {
  security_group_id = aws_security_group.task.id
  description       = "HTTPS to the interface VPC endpoints"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  cidr_ipv4         = var.vpc_cidr
}

# ゲートウェイ型エンドポイント経由で、S3 (イメージのレイヤー) へ HTTPS で出る
resource "aws_vpc_security_group_egress_rule" "task_to_s3" {
  security_group_id = aws_security_group.task.id
  description       = "HTTPS to S3 through the gateway VPC endpoint"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  prefix_list_id    = data.aws_ec2_managed_prefix_list.s3.id
}
