# ================================================================================
# Private Subnet
# ================================================================================

# インターネットに面していないサブネット
#   - 用途: ECS のタスクと、インターフェース型 VPC エンドポイントの ENI の配置先
#   - ALB は配置しない (パブリックサブネットに配置する)
#   - CIDR は 10 番目以降の /24 を使う (パブリックサブネットの 0 番目からと、番号で見分けられるようにするため)
resource "aws_subnet" "private" {
  for_each = { for i, az in local.azs : az => i }

  vpc_id            = aws_vpc.main.id
  availability_zone = each.key
  cidr_block        = cidrsubnet(var.vpc_cidr, 8, each.value + 10)

  tags = {
    Name = "${local.name_prefix}-private-${substr(each.key, -2, 2)}"
  }
}

# インターネットへの経路を持たないルートテーブル
#   - NAT を置かないため、0.0.0.0/0 の経路はない
#   - 外への通信は VPC エンドポイント経由だけになる (S3 のゲートウェイ型エンドポイントの経路は、このテーブルに追加される)
resource "aws_route_table" "private" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${local.name_prefix}-private-rt"
  }
}

resource "aws_route_table_association" "private" {
  for_each = aws_subnet.private

  subnet_id      = each.value.id
  route_table_id = aws_route_table.private.id
}
