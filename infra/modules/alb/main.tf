# ================================================================================
# Load Balancer
# ================================================================================

# インターネットに公開する ALB
#   - パブリックサブネットに置き、ターゲット (タスク) はプライベートサブネットに置く
#   - 不正なヘッダーを含むリクエストは、ターゲットに渡さずに破棄する
#   - 検証が終わったら壊す環境のため、削除保護はかけない
#   - trivy の AVD-AWS-0053 (ALB を公開している) は、公開するのが目的のため除外する
#trivy:ignore:AVD-AWS-0053
resource "aws_lb" "this" {
  name                       = "${local.name_prefix}-alb"
  load_balancer_type         = "application"
  internal                   = false
  subnets                    = var.public_subnet_ids
  security_groups            = [aws_security_group.alb.id]
  drop_invalid_header_fields = true
  enable_deletion_protection = false
}

# ================================================================================
# Target Groups
# ================================================================================

# ターゲットグループ (blue / green)
#   - ローリングでも 2 つ作っておき、構造を変えずにデプロイ方式を切り替えられるようにする
#   - Fargate (awsvpc) のタスクは ENI の IP で登録されるため、ip 型にする
#   - 登録解除の遅延は、アプリの終了待ち (20 秒) より長くする (既定の 300 秒は検証には長すぎる)
#   - ヘルスチェックは、切り替えを速く確かめるため、間隔を短く、判定の回数を少なくする
resource "aws_lb_target_group" "this" {
  for_each = toset(["blue", "green"])

  name                 = "${local.name_prefix}-${each.key}"
  target_type          = "ip"
  protocol             = "HTTP"
  port                 = var.target_port
  vpc_id               = var.vpc_id
  deregistration_delay = var.deregistration_delay

  health_check {
    path                = var.health_check_path
    matcher             = "200"
    interval            = 10
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 2
  }
}

# ================================================================================
# Listeners
# ================================================================================

locals {
  # HTTPS のリスナー
  #   - prod: 本番の入口
  #   - test: ブルー/グリーンなどで、切り替える前の新しい版を確かめる入口 (証明書は本番と同じ)
  https_listener_ports = {
    prod = 443
    test = 8443
  }
}

# 80 番は、443 番へリダイレクトするだけ
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"

    redirect {
      protocol    = "HTTPS"
      port        = "443"
      status_code = "HTTP_301"
    }
  }
}

# HTTPS のリスナー (443 / 8443)
#   - どのルールにも当たらないリクエストは、404 を返す
#   - ターゲットへの転送は、デフォルトのアクションではなく下のルールで行う
resource "aws_lb_listener" "https" {
  for_each = local.https_listener_ports

  load_balancer_arn = aws_lb.this.arn
  port              = each.value
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = var.certificate_arn

  default_action {
    type = "fixed-response"

    fixed_response {
      content_type = "text/plain"
      message_body = "not found"
      status_code  = "404"
    }
  }
}

# ターゲットへ転送するルール
#   - blue / green の 2 つのターゲットグループを、重み 1 : 0 で設定する
#   - ECS のブルー/グリーンは、デフォルトのアクションではなく、このルールの重みを書き換えて切り替える
#   - デプロイのたびに重みが変わるため、作ったあとの action の変更は Terraform で戻さない
resource "aws_lb_listener_rule" "forward" {
  for_each = local.https_listener_ports

  listener_arn = aws_lb_listener.https[each.key].arn
  priority     = 1

  condition {
    path_pattern {
      values = ["/*"]
    }
  }

  action {
    type = "forward"

    forward {
      target_group {
        arn    = aws_lb_target_group.this["blue"].arn
        weight = 1
      }

      target_group {
        arn    = aws_lb_target_group.this["green"].arn
        weight = 0
      }
    }
  }

  lifecycle {
    ignore_changes = [action]
  }
}
