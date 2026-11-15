# ================================================================================
# Hosted Zone
# ================================================================================

# レコードを作るホストゾーン
#   - tamaco-blog が管理しているゾーンを、名前で引いて使う (ブログの state は参照しない)
#   - このプロジェクトで作るのは、サブドメインのレコードと、証明書の検証レコードだけ
data "aws_route53_zone" "this" {
  name         = var.zone_name
  private_zone = false
}

# ================================================================================
# Certificate
# ================================================================================

# このプロジェクト専用の証明書 (ブログのワイルドカード証明書は使わない)
#   - DNS 検証で発行する。検証レコードは下で作る
#   - 作り直すときに、リスナーが証明書を失わないよう、新しい証明書を先に作る
resource "aws_acm_certificate" "this" {
  domain_name       = var.domain_name
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

# 証明書の検証レコード
#   - ACM が指定する CNAME をゾーンに作ると、ACM がそれを見て発行する
#   - 同じドメインの証明書を作り直すと同じ CNAME になるため、上書きを許可する
resource "aws_route53_record" "validation" {
  for_each = {
    for o in aws_acm_certificate.this.domain_validation_options : o.domain_name => {
      name   = o.resource_record_name
      type   = o.resource_record_type
      record = o.resource_record_value
    }
  }

  zone_id         = data.aws_route53_zone.this.zone_id
  name            = each.value.name
  type            = each.value.type
  records         = [each.value.record]
  ttl             = 300
  allow_overwrite = true
}

# 証明書が発行されるまで待つ
#   - 証明書の ARN はここから出力し、発行前の証明書をリスナーに付けないようにする
resource "aws_acm_certificate_validation" "this" {
  certificate_arn         = aws_acm_certificate.this.arn
  validation_record_fqdns = [for r in aws_route53_record.validation : r.fqdn]
}

# ================================================================================
# Record
# ================================================================================

# サブドメインを ALB に向ける
#   - エイリアスレコードにし、ALB の IP が変わっても追従させる
resource "aws_route53_record" "alb" {
  zone_id = data.aws_route53_zone.this.zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = var.alb_dns_name
    zone_id                = var.alb_zone_id
    evaluate_target_health = true
  }
}
