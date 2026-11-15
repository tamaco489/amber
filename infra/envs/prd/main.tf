module "network" {
  source = "../../modules/network"

  env                        = var.env
  vpc_cidr                   = var.vpc_cidr
  enable_interface_endpoints = var.enable_interface_endpoints
}

module "github_oidc" {
  source = "../../modules/github_oidc"

  env                    = var.env
  github_oidc_sub_prefix = var.github_oidc_sub_prefix
  tfstate_bucket         = var.tfstate_bucket
  tfstate_key            = var.tfstate_key
}

module "ecr_api" {
  source = "../../modules/ecr"

  env  = var.env
  name = "api"
}

# ALB と DNS は、enable_alb で作るかどうかを切り替える (使わない間の費用を止めるため)
module "alb" {
  source = "../../modules/alb"
  count  = var.enable_alb ? 1 : 0

  env               = var.env
  vpc_id            = module.network.vpc_id
  vpc_cidr          = var.vpc_cidr
  public_subnet_ids = module.network.public_subnet_ids
  certificate_arn   = module.dns[0].certificate_arn
}

module "dns" {
  source = "../../modules/dns"
  count  = var.enable_alb ? 1 : 0

  zone_name    = var.zone_name
  domain_name  = var.domain_name
  alb_dns_name = module.alb[0].dns_name
  alb_zone_id  = module.alb[0].zone_id
}
