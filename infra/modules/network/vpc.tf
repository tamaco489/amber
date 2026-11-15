# ================================================================================
# VPC
# ================================================================================

# ECS のタスクなどを配置する VPC
#   - enable_dns_support: VPC の中の名前解決に、AWS の DNS サーバー (Route 53 Resolver) を使えるようにする
#   - enable_dns_hostnames: VPC の中のリソースに、DNS のホスト名を割り当てる
#   - 2 つとも有効にしないと、インターフェース型 VPC エンドポイントのプライベート DNS が使えない (ECR などの名前が、エンドポイントの ENI の IP に解決されない)
resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name = "${local.name_prefix}-vpc"
  }
}

# VPC の既定のセキュリティグループ (VPC を作ると AWS が自動で作り、削除できない)
#   - 初期状態では、同じセキュリティグループ同士の通信と、すべての外向きの通信を許可している
#   - このリソースで Terraform の管理下に置き、ingress / egress を書かないことで、ルールをすべて削除する
#   - 各リソースには用途ごとのセキュリティグループを付けるため、これは使わない
#   - セキュリティグループを指定し忘れて、既定のものが付いてしまっても、通信できない状態にしておくため
resource "aws_default_security_group" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${local.name_prefix}-default-sg"
  }
}

# インターネットゲートウェイ (VPC とインターネットの間の出入口)
#   - 用途: パブリックサブネットの ALB が、インターネットからのリクエストを受けるため
#   - 経路はパブリックのルートテーブルにだけ追加する (プライベートサブネットからは使わない)
#   - NAT ゲートウェイとは別物で、プライベートサブネットのタスクがインターネットに出る手段にはならない
resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${local.name_prefix}-igw"
  }
}
