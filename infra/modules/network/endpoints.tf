locals {
  # enable_interface_endpoints で、インターフェース型のエンドポイントを作るかを切り替える
  #   - true: ecr.api / ecr.dkr / logs を作り、VPC の中ではこれらの名前がエンドポイントの ENI の IP に解決される
  #   - false: 3 つとも削除され、VPC の中でも公開 IP に解決される。NAT がないため ECR と CloudWatch Logs に届かない
  # S3 のゲートウェイ型は、この値に関係なく残る
  interface_endpoint_services = var.enable_interface_endpoints ? toset(["ecr.api", "ecr.dkr", "logs"]) : toset([])
}

# ECR のイメージのレイヤーは S3 に置かれているため、pull には S3 のエンドポイントも要る
# ゲートウェイ型は無料なので、enable_interface_endpoints に関係なく常に作る
resource "aws_vpc_endpoint" "s3" {
  vpc_id            = aws_vpc.main.id
  service_name      = "com.amazonaws.${data.aws_region.current.region}.s3"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = [aws_route_table.private.id]

  tags = {
    Name = "${local.name_prefix}-s3-endpoint"
  }
}

# 料金は「エンドポイント 1 個 × AZ 1 つ」ごとの時間課金 (東京で 1 時間 0.014 ドル)
# private_dns_enabled により、VPC の中ではサービスの通常の名前がこのエンドポイントの ENI の IP に解決される
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

# エンドポイントは接続を受けるだけなので、外向きのルールは付けない
resource "aws_security_group" "vpc_endpoint" {
  name        = "${local.name_prefix}-vpc-endpoint-sg"
  description = "Allow HTTPS from the private subnets to the interface VPC endpoints"
  vpc_id      = aws_vpc.main.id

  tags = {
    Name = "${local.name_prefix}-vpc-endpoint-sg"
  }
}

resource "aws_vpc_security_group_ingress_rule" "vpc_endpoint_https" {
  for_each = aws_subnet.private

  security_group_id = aws_security_group.vpc_endpoint.id
  description       = "HTTPS from private subnet ${each.key}"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  cidr_ipv4         = each.value.cidr_block
}
