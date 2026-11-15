# ================================================================================
# Data Sources
# ================================================================================

# サブネットを置く AZ の候補
#   - zone-type で、通常の AZ だけに絞る (Local Zones や Wavelength Zones を除く)
#   - 先頭から az_count 個を使う (locals.tf の azs)
data "aws_availability_zones" "available" {
  state = "available"

  filter {
    name   = "zone-type"
    values = ["availability-zone"]
  }
}

# VPC エンドポイントのサービス名 (com.amazonaws.<リージョン>.<サービス>) を組み立てるため
data "aws_region" "current" {}
