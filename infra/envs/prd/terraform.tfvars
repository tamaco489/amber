env      = "prd"
region   = "ap-northeast-1"
vpc_cidr = "10.2.0.0/16"

github_oidc_sub_prefix = "repo:tamaco489@189016912/amber@1457865823"
tfstate_bucket         = "prd-amber-tfstate"
tfstate_key            = "envs/prd/terraform.tfstate"

# ECS のタスクができるまでは使わないため、false にして費用を止めておく
# タスクを動かす前に true に戻して apply する
enable_interface_endpoints = false
