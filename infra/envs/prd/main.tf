module "network" {
  source = "../../modules/network"

  env                        = var.env
  vpc_cidr                   = var.vpc_cidr
  enable_interface_endpoints = var.enable_runtime
}

module "github_oidc" {
  source = "../../modules/github_oidc"

  env                    = var.env
  github_oidc_sub_prefix = var.github_oidc_sub_prefix
  tfstate_bucket         = var.tfstate_bucket
  tfstate_key            = var.tfstate_key
  ecr_repository_arn     = module.ecr_api.repository_arn
}

module "ecr_api" {
  source = "../../modules/ecr"

  env  = var.env
  name = "api"
}

# 時間課金のリソースは、enable_runtime で作るかどうかを切り替える (使わない間の費用を止めるため)
module "alb" {
  source = "../../modules/alb"
  count  = var.enable_runtime ? 1 : 0

  env               = var.env
  vpc_id            = module.network.vpc_id
  vpc_cidr          = var.vpc_cidr
  public_subnet_ids = module.network.public_subnet_ids
  certificate_arn   = module.dns[0].certificate_arn
}

module "dns" {
  source = "../../modules/dns"
  count  = var.enable_runtime ? 1 : 0

  zone_name    = var.zone_name
  domain_name  = var.domain_name
  alb_dns_name = module.alb[0].dns_name
  alb_zone_id  = module.alb[0].zone_id
}

# 最初のタスク定義で使う api のイメージ (手元から bootstrap のタグで push したもの) のダイジェストを、ECR から引く
#   - ECR にイメージがなければ、plan の時点でエラーになる
#   - module.ecs の中に置くと、module.ecs の depends_on のために apply まで読まれず、plan で気づけないため、ここに置く
data "aws_ecr_image" "api_initial" {
  count = var.enable_runtime ? 1 : 0

  repository_name = module.ecr_api.repository_name
  image_tag       = "bootstrap"
}

module "ecs" {
  source = "../../modules/ecs"
  count  = var.enable_runtime ? 1 : 0

  env                   = var.env
  vpc_id                = module.network.vpc_id
  vpc_cidr              = var.vpc_cidr
  private_subnet_ids    = module.network.private_subnet_ids
  alb_security_group_id = module.alb[0].security_group_id
  alb_arn_suffix        = module.alb[0].arn_suffix
  target_group_arn      = module.alb[0].target_group_arns["blue"]
  image                 = "${module.ecr_api.repository_url}@${data.aws_ecr_image.api_initial[0].image_digest}"

  # タスクはインターフェース型エンドポイント経由でイメージを pull し、ログを出す
  # サービスはエンドポイントを参照していないため、エンドポイントができてからタスクを起動するよう、順序を明示する
  depends_on = [module.network]
}
