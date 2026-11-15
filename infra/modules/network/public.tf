# ================================================================================
# Public Subnet
# ================================================================================

# インターネットに面したサブネット
#   - 用途: インターネットからのリクエストを受ける ALB の配置先
#   - ECS のタスクは配置しない (プライベートサブネットに配置する)
#   - パブリック IP の自動付与 (map_public_ip_on_launch) は有効にしない。ALB は自分でパブリック IP を持つため
resource "aws_subnet" "public" {
  for_each = { for i, az in local.azs : az => i }

  vpc_id            = aws_vpc.main.id
  availability_zone = each.key
  cidr_block        = cidrsubnet(var.vpc_cidr, 8, each.value)

  tags = {
    Name = "${local.name_prefix}-public-${substr(each.key, -2, 2)}"
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${local.name_prefix}-public-rt"
  }
}

resource "aws_route" "public_internet" {
  route_table_id         = aws_route_table.public.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.main.id
}

resource "aws_route_table_association" "public" {
  for_each = aws_subnet.public

  subnet_id      = each.value.id
  route_table_id = aws_route_table.public.id
}
