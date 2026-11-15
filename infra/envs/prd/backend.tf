# バケットは Terraform の外で AWS CLI を使って作っている
# 認証はプロファイルを書かず、手元は AWS_PROFILE、CI は OIDC の一時クレデンシャルで渡す
terraform {
  backend "s3" {
    bucket       = "prd-amber-tfstate"
    key          = "envs/prd/terraform.tfstate"
    region       = "ap-northeast-1"
    encrypt      = true
    use_lockfile = true
  }
}
