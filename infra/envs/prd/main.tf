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
