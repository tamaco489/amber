locals {
  # true: ecr.api / ecr.dkr / logs のエンドポイントを作り、VPC の中からはこれらの名前がエンドポイントの ENI の IP に解決される (プライベート DNS)
  # false: 空の集合になり、3 つのエンドポイントと ENI が削除される
  # false: VPC の中でも公開 IP に解決され、NAT がないため ECR と CloudWatch Logs に届かなくなる
  # S3 のゲートウェイ型エンドポイントはこの値に関係なく残る
  interface_endpoint_services = var.enable_interface_endpoints ? toset(["ecr.api", "ecr.dkr", "logs"]) : toset([])
}

# イメージのレイヤーの実体は S3 から取得するため、ECR を使うにはゲートウェイ型のエンドポイントも要る
# ゲートウェイ型は無料なので、インターフェース型を消す設定でも残す
resource "aws_vpc_endpoint" "s3" {
  vpc_id            = aws_vpc.main.id
  service_name      = "com.amazonaws.${data.aws_region.current.region}.s3"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = [aws_route_table.private.id]

  tags = {
    Name = "${local.name_prefix}-s3-endpoint"
  }
}

# インターフェース型は「1 個 × 1 AZ」ごとに時間課金されるため、使わない間は enable_interface_endpoints で消せるようにする
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

# エンドポイントは受けるだけなので、外向きのルールは付けない
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
