env      = "prd"
region   = "ap-northeast-1"
vpc_cidr = "10.2.0.0/16"

# インターフェース型の VPC エンドポイントは 3 個 × 2 AZ で 1 時間 0.084 ドル (月 60 ドル前後) かかる
# ECS のタスクができるまでは使う相手がいないため false にして消しておく
# タスクを動かす前に true に戻して apply する (戻さないとタスクがイメージを pull できず、ログも出せない)
enable_interface_endpoints = false
