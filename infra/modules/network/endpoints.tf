# ================================================================================
# VPC Endpoints
# ================================================================================

locals {
  # インターフェース型エンドポイントを作るサービスの一覧 (enable_interface_endpoints で切り替える)
  #   - true: ecr.api / ecr.dkr / logs を作り、VPC の中ではこれらの名前がエンドポイントの ENI の IP に解決される
  #   - false: 空の集合になり、3 つとも削除される
  #   - false の間は VPC の中でも公開 IP に解決され、NAT がないため ECR と CloudWatch Logs に届かない
  #   - S3 のゲートウェイ型エンドポイントは、この値に関係なく残る
  interface_endpoint_services = var.enable_interface_endpoints ? toset(["ecr.api", "ecr.dkr", "logs"]) : toset([])
}

# S3 のゲートウェイ型エンドポイント
#   - 用途: ECR のイメージの pull (イメージのレイヤーの実体は S3 に置かれている)
#   - プライベートのルートテーブルに、S3 への経路が追加される
#   - 料金はかからないため、enable_interface_endpoints に関係なく常に作る
resource "aws_vpc_endpoint" "s3" {
  vpc_id            = aws_vpc.main.id
  service_name      = "com.amazonaws.${data.aws_region.current.region}.s3"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = [aws_route_table.private.id]

  tags = {
    Name = "${local.name_prefix}-s3-endpoint"
  }
}

# インターフェース型エンドポイント (ecr.api / ecr.dkr / logs)
#   - 用途: ECS のタスクが使う AWS のサービスへの入口
#     - ecr.api: ECR の認証トークンの取得
#     - ecr.dkr: イメージのマニフェストとレイヤーの取得 (レイヤーの実体は S3 から取得する)
#     - logs: CloudWatch Logs へのログの出力
#   - プライベートサブネットごとに ENI を 1 つ配置する
#   - private_dns_enabled: VPC の中では、サービスの通常の名前がこのエンドポイントの ENI の IP に解決される
#   - 料金は「エンドポイント 1 個 × AZ 1 つ」ごとの時間課金 (東京で 1 時間 0.014 ドル)
# DOC: https://aws.amazon.com/privatelink/pricing/
resource "aws_vpc_endpoint" "interface" {
  for_each = local.interface_endpoint_services

  vpc_id              = aws_vpc.main.id
  service_name        = "com.amazonaws.${data.aws_region.current.region}.${each.key}"
  vpc_endpoint_type   = "Interface"
  private_dns_enabled = true
  subnet_ids          = [for subnet in aws_subnet.private : subnet.id]
  security_group_ids  = [aws_security_group.vpc_endpoint.id]

  tags = {
    Name = "${local.name_prefix}-${replace(each.key, ".", "-")}-endpoint"
  }
}

# インターフェース型エンドポイントの ENI に付けるセキュリティグループ
#   - 内向き: プライベートサブネットからの HTTPS (443) だけを許可する
#   - 外向き: エンドポイントは接続を受けるだけなので、ルールは付けない
resource "aws_security_group" "vpc_endpoint" {
  name        = "${local.name_prefix}-vpc-endpoint-sg"
  description = "Allow HTTPS from the private subnets to the interface VPC endpoints"
  vpc_id      = aws_vpc.main.id

  tags = {
    Name = "${local.name_prefix}-vpc-endpoint-sg"
  }
}

# 上のセキュリティグループの内向きのルール (プライベートサブネットからの HTTPS)
#   - プライベートサブネットごとに 1 つ作り、送信元をそのサブネットの CIDR に絞る
#   - 送信元は ECS のタスクなど、エンドポイント経由で AWS のサービスを呼ぶもの
#   - AWS のサービスの API は HTTPS なので、443 番だけを許可する
#   - 呼ぶ側にも、外向きの 443 番を許可するセキュリティグループが要る (両方そろって初めて通信できる)
#   - aws_security_group の ingress ブロックではなく、このリソースで 1 ルールずつ定義する (AWS provider が推奨する書き方)
resource "aws_vpc_security_group_ingress_rule" "vpc_endpoint_https" {
  for_each = aws_subnet.private

  security_group_id = aws_security_group.vpc_endpoint.id
  description       = "HTTPS from private subnet ${each.key}"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  cidr_ipv4         = each.value.cidr_block
}
