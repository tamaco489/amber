# ================================================================================
# Security Group
# ================================================================================

# テスト用リスナー (8443 番) への接続を許可する CIDR の一覧
#   - 公開リポジトリに自分の IP を書かないため、Terraform の外 (just infra::allow-my-ip) で作って名前で引く
#   - 名前は <env>-amber-test-listener-allowed (justfile のレシピと揃える)
#   - IP が変わってもリストを書き換えるだけで済み、apply は要らない
data "aws_ec2_managed_prefix_list" "test_listener" {
  name = "${local.name_prefix}-test-listener-allowed"
}

# ALB のセキュリティグループ
#   - 内向き: 80 / 443 はインターネット全体から、8443 はプレフィックスリストの CIDR からだけ
#   - 外向き: VPC の中のターゲット (タスク) のアプリのポートだけ
resource "aws_security_group" "alb" {
  name        = "${local.name_prefix}-alb-sg"
  description = "Allow HTTP and HTTPS from the internet and the test listener from allowed CIDRs"
  vpc_id      = var.vpc_id

  tags = {
    Name = "${local.name_prefix}-alb-sg"
  }
}

# インターネットに公開する本番の入口 (80 は 443 へリダイレクトするだけ)
#   - 公開する ALB のため、送信元を絞らない
resource "aws_vpc_security_group_ingress_rule" "public" {
  for_each = toset(["80", "443"])

  security_group_id = aws_security_group.alb.id
  description       = "Port ${each.key} from the internet"
  ip_protocol       = "tcp"
  from_port         = tonumber(each.key)
  to_port           = tonumber(each.key)
  cidr_ipv4         = "0.0.0.0/0"
}

# テスト用リスナー
#   - ブルー/グリーンなどで、切り替える前の新しい版を確かめる入口のため、自分の IP だけに絞る
resource "aws_vpc_security_group_ingress_rule" "test_listener" {
  security_group_id = aws_security_group.alb.id
  description       = "Test listener from allowed CIDRs"
  ip_protocol       = "tcp"
  from_port         = 8443
  to_port           = 8443
  prefix_list_id    = data.aws_ec2_managed_prefix_list.test_listener.id
}

# ターゲットへの転送とヘルスチェック
#   - タスクは VPC の中にあるため、宛先を VPC の CIDR とアプリのポートに絞る
resource "aws_vpc_security_group_egress_rule" "targets" {
  security_group_id = aws_security_group.alb.id
  description       = "Application port to the targets in the VPC"
  ip_protocol       = "tcp"
  from_port         = var.target_port
  to_port           = var.target_port
  cidr_ipv4         = var.vpc_cidr
}
