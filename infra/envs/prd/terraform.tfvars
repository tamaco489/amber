env      = "prd"
region   = "ap-northeast-1"
vpc_cidr = "10.2.0.0/16"

github_oidc_sub_prefix = "repo:tamaco489@189016912/amber@1457865823"
tfstate_bucket         = "prd-amber-tfstate"
tfstate_key            = "envs/prd/terraform.tfstate"

# ホストゾーンは tamaco-blog が管理しているものを使い、サブドメインだけをこちらで作る
zone_name   = "tamaco-blog.jp"
domain_name = "ecs-lab.tamaco-blog.jp"

# 時間課金のリソース (ALB と DNS、インターフェース型エンドポイント、ECS) を、まとめて作るか消すかを切り替える
# 検証するときだけ true にして apply し、終わったら false に戻して apply する
enable_runtime = true
