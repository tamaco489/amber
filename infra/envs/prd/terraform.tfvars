env      = "prd"
region   = "ap-northeast-1"
vpc_cidr = "10.2.0.0/16"

# ECS のタスクができるまでは使わないため、false にして費用を止めておく
# タスクを動かす前に true に戻して apply する
enable_interface_endpoints = false
