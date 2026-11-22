# ================================================================================
# Repository
# ================================================================================

# アプリのイメージを置くリポジトリ
#   - タグは上書き禁止 (同じタグで push し直すと拒否される)
#   - デプロイではタグではなくダイジェストを指定するため、タグは目印として使う
#   - push 時に基本スキャンを実行し、結果はコンソールか describe-image-scan-findings で見る
#   - 検証が終わったら壊す環境のため、イメージが残っていても destroy できるようにする
resource "aws_ecr_repository" "this" {
  name                 = "${local.name_prefix}-${var.name}"
  image_tag_mutability = "IMMUTABLE"
  force_delete         = true

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "AES256"
  }
}

# ================================================================================
# Lifecycle Policy
# ================================================================================

# 古いイメージを消す
#   - タグなしのイメージは、push から 1 日で消す
#   - CD が push するバージョンのイメージ (v* のタグ) は、新しいものから keep_image_count 個だけ残す (ロールバックの検証で数世代前に戻せれば足りる)
#   - それ以外のタグ (最初のタスク定義で使う bootstrap) は消さない。消すと、enable_runtime で作り直したときにタスクが起動できなくなる
resource "aws_ecr_lifecycle_policy" "this" {
  repository = aws_ecr_repository.this.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Expire untagged images 1 day after push"
        selection = {
          tagStatus   = "untagged"
          countType   = "sinceImagePushed"
          countUnit   = "days"
          countNumber = 1
        }
        action = {
          type = "expire"
        }
      },
      {
        rulePriority = 2
        description  = "Keep only the latest ${var.keep_image_count} release images (v*)"
        selection = {
          tagStatus      = "tagged"
          tagPatternList = ["v*"]
          countType      = "imageCountMoreThan"
          countNumber    = var.keep_image_count
        }
        action = {
          type = "expire"
        }
      },
    ]
  })
}
