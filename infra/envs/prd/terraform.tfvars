env      = "prd"
region   = "ap-northeast-1"
vpc_cidr = "10.2.0.0/16"

github_oidc_sub_prefix = "repo:tamaco489@189016912/amber@1457865823"
tfstate_bucket         = "prd-amber-tfstate"
tfstate_key            = "envs/prd/terraform.tfstate"

# ECS のタスクができるまでは使わないため、false にして費用を止めておく
# タスクを動かす前に true に戻して apply する
enable_interface_endpoints = false

# ホストゾーンは tamaco-blog が管理しているものを使い、サブドメインだけをこちらで作る
zone_name   = "tamaco-blog.jp"
domain_name = "ecs-lab.tamaco-blog.jp"

# ALB は時間課金のため、使わない間は false にして、DNS のレコードと証明書ごと消しておく
# ECS のタスクを動かす前に true に戻して apply する
enable_alb = false
