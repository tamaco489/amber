module "network" {
  source = "../../modules/network"

  env                        = var.env
  vpc_cidr                   = var.vpc_cidr
  enable_interface_endpoints = var.enable_interface_endpoints
}
